// Command service runs subgen: it serves per-client mihomo (Clash.Meta)
// subscription configs and a small admin panel. Bootstrap settings (listener,
// TLS, secret, admin creds, db path) come from the environment / a local .env
// file; operational config (nodes, rules, rule-providers, base YAML) lives in
// SQLite and is edited via the admin panel.
//
// This file is the composition root: it loads config, opens the repositories,
// constructs the clients/services/handlers and wires the gorilla/mux router.
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gorilla/mux"

	"github.com/postlog/subgen/internal/cert"
	"github.com/postlog/subgen/internal/clients/xui"
	"github.com/postlog/subgen/internal/config"
	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/handlers/config_api"
	"github.com/postlog/subgen/internal/handlers/config_customs"
	"github.com/postlog/subgen/internal/handlers/config_save"
	"github.com/postlog/subgen/internal/handlers/config_schema"
	"github.com/postlog/subgen/internal/handlers/custom_create"
	"github.com/postlog/subgen/internal/handlers/custom_delete"
	"github.com/postlog/subgen/internal/handlers/healthz"
	"github.com/postlog/subgen/internal/handlers/login"
	"github.com/postlog/subgen/internal/handlers/logout"
	"github.com/postlog/subgen/internal/handlers/node_delete"
	"github.com/postlog/subgen/internal/handlers/node_save"
	"github.com/postlog/subgen/internal/handlers/nodes_api"
	"github.com/postlog/subgen/internal/handlers/provider_check"
	"github.com/postlog/subgen/internal/handlers/rules"
	"github.com/postlog/subgen/internal/handlers/sub"
	"github.com/postlog/subgen/internal/handlers/user_create"
	"github.com/postlog/subgen/internal/handlers/user_delete"
	"github.com/postlog/subgen/internal/handlers/user_edit"
	"github.com/postlog/subgen/internal/handlers/user_recreate"
	"github.com/postlog/subgen/internal/handlers/users_api"
	"github.com/postlog/subgen/internal/handlers/web"
	"github.com/postlog/subgen/internal/repository"
	"github.com/postlog/subgen/internal/repository/configs"
	"github.com/postlog/subgen/internal/repository/nodes"
	"github.com/postlog/subgen/internal/repository/routing"
	"github.com/postlog/subgen/internal/repository/users"
	"github.com/postlog/subgen/internal/service/fleet"
	"github.com/postlog/subgen/internal/service/provisioning"
	"github.com/postlog/subgen/internal/service/ruleset"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	envFile := flag.String("env", ".env", "path to .env file (KEY=VALUE); the process environment takes precedence")

	flag.Parse()

	cfg, err := config.Load(*envFile)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	if dir := filepath.Dir(cfg.DBPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("db dir: %w", err)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := repository.Open(ctx, cfg.DBPath)
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}
	defer db.Close()

	usersRepo := users.New(db)
	nodesRepo := nodes.New(db)
	routingRepo := routing.New(db)
	configsRepo := configs.New(db, routingRepo)

	// The mirror serves rule-provider files referenced by any config (base + custom).
	provs, err := routingRepo.AllRuleProviders(ctx)
	if err != nil {
		return fmt.Errorf("rule providers: %w", err)
	}

	// Wiring: clients → services → handlers (data flows bottom-up).
	xc := xui.New()
	fleetSvc := fleet.New(xc, nodesRepo)
	prov := provisioning.New(usersRepo, nodesRepo, xc)

	mirror := ruleset.New(provs)
	go mirror.Run(ctx)

	router := buildRouter(cfg, usersRepo, nodesRepo, routingRepo, configsRepo, fleetSvc, prov, mirror)

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serveErr := make(chan error, 1)

	go func() { serveErr <- serve(cfg, srv) }()

	select {
	case <-ctx.Done():
		slog.Info("shutting down")
	case err := <-serveErr:
		return err
	}

	shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = srv.Shutdown(shCtx)

	return nil
}

// serve starts the HTTP(S) listener and blocks until it stops; a clean shutdown
// (http.ErrServerClosed) returns nil.
func serve(cfg config.Config, srv *http.Server) error {
	if cfg.TLSEnabled() {
		certs, err := cert.NewReloader(cfg.TLSCert, cfg.TLSKey)
		if err != nil {
			return fmt.Errorf("tls: %w", err)
		}

		srv.TLSConfig = &tls.Config{GetCertificate: certs.GetCertificate, MinVersion: tls.VersionTLS12}

		slog.Info("subgen listening", "addr", cfg.Listen, "tls", true)

		if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("serve: %w", err)
		}

		return nil
	}

	slog.Info("subgen listening", "addr", cfg.Listen, "tls", false)

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("serve: %w", err)
	}

	return nil
}

// buildRouter constructs the gorilla/mux router, wiring each per-action handler
// with the concrete dependencies it declares.
func buildRouter(cfg config.Config, usersRepo *users.Repository, nodesRepo *nodes.Repository, routingRepo *routing.Repository, configsRepo *configs.Repository, fleetSvc *fleet.Service, prov *provisioning.Service, mirror *ruleset.Mirror) http.Handler {
	r := mux.NewRouter()

	// Per-engine subscription renderers, keyed by config kind. Adding an engine = a
	// new engineRenderer + one entry here; the route and handler don't change.
	renderers := map[entity.ConfigKind]sub.EngineRenderer{
		entity.ConfigKindMihomo: sub.NewMihomoRenderer(routingRepo, cfg.PublicBase, cfg.Filename),
	}

	r.Handle("/sub/{kind}/{token}", sub.New(
		usersRepo, fleetSvc, configsRepo, renderers,
		cfg.Secret, cfg.ProfileTitle, cfg.ProfileUpdateInterval,
	)).Methods(http.MethodGet)
	r.Handle("/rules/{file}", rules.New(mirror)).Methods(http.MethodGet)
	r.Handle("/healthz", healthz.New()).Methods(http.MethodGet)

	if cfg.AdminEnabled() {
		mountAdmin(r, cfg, usersRepo, nodesRepo, routingRepo, configsRepo, fleetSvc, prov)
	}

	return r
}

func mountAdmin(r *mux.Router, cfg config.Config, usersRepo *users.Repository, nodesRepo *nodes.Repository, routingRepo *routing.Repository, configsRepo *configs.Repository, fleetSvc *fleet.Service, prov *provisioning.Service) {
	sess := web.NewSession(cfg.Secret)
	ra := sess.RequireAdmin

	r.PathPrefix("/admin/static/").Handler(web.StaticHandler(cfg.StaticDir))
	r.Handle("/admin/login", login.New(sess, cfg.AdminUser, cfg.AdminPassword, cfg.StaticDir)).Methods(http.MethodGet, http.MethodPost)
	r.Handle("/admin/logout", logout.New(sess)).Methods(http.MethodGet)

	// JSON read API (consumed by the Vue SPA).
	r.HandleFunc("/admin/api/users", ra(users_api.New(usersRepo, fleetSvc, cfg.Secret, cfg.PublicBase).ServeHTTP)).Methods(http.MethodGet)
	r.HandleFunc("/admin/api/nodes", ra(nodes_api.New(nodesRepo).ServeHTTP)).Methods(http.MethodGet)

	// mihomo config: read / schema / save / custom-config management, grouped under
	// /admin/api/config/mihomo. Without ?user / userId the scope is the base config;
	// with it, a user's custom config.
	r.HandleFunc("/admin/api/config/mihomo", ra(config_api.New(configsRepo, routingRepo).ServeHTTP)).Methods(http.MethodGet)
	r.HandleFunc("/admin/api/config/mihomo/schema", ra(config_schema.New().ServeHTTP)).Methods(http.MethodGet)
	r.HandleFunc("/admin/api/config/mihomo/save", ra(config_save.New(configsRepo, routingRepo).ServeHTTP)).Methods(http.MethodPost)
	r.HandleFunc("/admin/api/config/mihomo/customs", ra(config_customs.New(configsRepo, usersRepo).ServeHTTP)).Methods(http.MethodGet)
	r.HandleFunc("/admin/api/config/mihomo/custom/create", ra(custom_create.New(configsRepo).ServeHTTP)).Methods(http.MethodPost)
	r.HandleFunc("/admin/api/config/mihomo/custom/delete", ra(custom_delete.New(configsRepo).ServeHTTP)).Methods(http.MethodPost)
	r.HandleFunc("/admin/api/config/mihomo/provider/check", ra(provider_check.New(ruleset.NewChecker()).ServeHTTP)).Methods(http.MethodPost)

	// JSON mutations.
	r.HandleFunc("/admin/api/users/create", ra(user_create.New(prov).ServeHTTP)).Methods(http.MethodPost)
	r.HandleFunc("/admin/api/users/edit", ra(user_edit.New(prov).ServeHTTP)).Methods(http.MethodPost)
	r.HandleFunc("/admin/api/users/delete", ra(user_delete.New(prov).ServeHTTP)).Methods(http.MethodPost)
	r.HandleFunc("/admin/api/users/recreate", ra(user_recreate.New(prov).ServeHTTP)).Methods(http.MethodPost)
	r.HandleFunc("/admin/api/nodes/save", ra(node_save.New(nodesRepo, routingRepo).ServeHTTP)).Methods(http.MethodPost)
	r.HandleFunc("/admin/api/nodes/delete", ra(node_delete.New(nodesRepo, routingRepo).ServeHTTP)).Methods(http.MethodPost)

	// SPA shell: serve index.html for /admin and any other admin GET (client-side views).
	shell := ra(func(w http.ResponseWriter, _ *http.Request) { web.ServePage(w, cfg.StaticDir, "index.html") })
	r.PathPrefix("/admin").Methods(http.MethodGet).HandlerFunc(shell)
}
