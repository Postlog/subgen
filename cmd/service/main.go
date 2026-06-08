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
	"github.com/postlog/subgen/internal/handlers/api"
	"github.com/postlog/subgen/internal/handlers/config_customs"
	"github.com/postlog/subgen/internal/handlers/config_get"
	"github.com/postlog/subgen/internal/handlers/config_save"
	"github.com/postlog/subgen/internal/handlers/config_schema"
	"github.com/postlog/subgen/internal/handlers/custom_create"
	"github.com/postlog/subgen/internal/handlers/custom_delete"
	"github.com/postlog/subgen/internal/handlers/healthz"
	"github.com/postlog/subgen/internal/handlers/login"
	"github.com/postlog/subgen/internal/handlers/logout"
	"github.com/postlog/subgen/internal/handlers/node_delete"
	"github.com/postlog/subgen/internal/handlers/node_save"
	"github.com/postlog/subgen/internal/handlers/nodes_get"
	"github.com/postlog/subgen/internal/handlers/provider_check"
	"github.com/postlog/subgen/internal/handlers/rules"
	"github.com/postlog/subgen/internal/handlers/sub"
	"github.com/postlog/subgen/internal/handlers/user_create"
	"github.com/postlog/subgen/internal/handlers/user_delete"
	"github.com/postlog/subgen/internal/handlers/user_edit"
	"github.com/postlog/subgen/internal/handlers/user_recreate"
	"github.com/postlog/subgen/internal/handlers/users_get"
	"github.com/postlog/subgen/internal/handlers/web"
	"github.com/postlog/subgen/internal/oas"
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

	router, err := buildRouter(cfg, usersRepo, nodesRepo, routingRepo, configsRepo, fleetSvc, prov, mirror)
	if err != nil {
		return fmt.Errorf("router: %w", err)
	}

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
// with the concrete dependencies it declares. Migrated operations are served by the
// ogen server (the api composite); the rest stay on the legacy per-action handlers
// until they migrate too.
func buildRouter(cfg config.Config, usersRepo *users.Repository, nodesRepo *nodes.Repository, routingRepo *routing.Repository, configsRepo *configs.Repository, fleetSvc *fleet.Service, prov *provisioning.Service, mirror *ruleset.Mirror) (http.Handler, error) {
	r := mux.NewRouter()

	// Per-engine subscription renderers, keyed by config kind. Adding an engine = a
	// new engineRenderer + one entry here; the route and handler don't change.
	renderers := map[entity.ConfigKind]sub.EngineRenderer{
		entity.ConfigKindMihomo: sub.NewMihomoRenderer(routingRepo, cfg.PublicBase, cfg.Filename),
	}

	sess := web.NewSession(cfg.Secret)

	// The login handler is shared: the ogen Login operation (POST /admin/api/login) and
	// the static login PAGE (GET /admin/login, served via its ServeHTTP in mountAdmin).
	loginHandler := login.New(sess, cfg.AdminUser, cfg.AdminPassword, cfg.StaticDir)

	// The ogen server (api composite) — forwards every operation to its per-action
	// handler, with the admin session cookie + idiomatic errors handled inside. Mounted
	// per-path on the router below.
	composite := api.New(sess, api.Handlers{
		Healthz: healthz.New(),
		Sub: sub.New(
			usersRepo, fleetSvc, configsRepo, renderers,
			cfg.Secret, cfg.ProfileTitle, cfg.ProfileUpdateInterval,
		),
		Rules:  rules.New(mirror),
		Login:  loginHandler,
		Logout: logout.New(sess),

		UsersGet:     users_get.New(usersRepo, fleetSvc, cfg.Secret, cfg.PublicBase),
		UserCreate:   user_create.New(prov),
		UserEdit:     user_edit.New(prov),
		UserDelete:   user_delete.New(prov),
		UserRecreate: user_recreate.New(prov),
		NodesGet:     nodes_get.New(nodesRepo),
		NodeSave:     node_save.New(nodesRepo, routingRepo),
		NodeDelete:   node_delete.New(nodesRepo, routingRepo),

		ConfigGet:     config_get.New(configsRepo, routingRepo),
		ConfigSchema:  config_schema.New(),
		ConfigCustoms: config_customs.New(configsRepo, usersRepo),
		ConfigSave:    config_save.New(configsRepo, routingRepo),
		CustomCreate:  custom_create.New(configsRepo),
		CustomDelete:  custom_delete.New(configsRepo),
		ProviderCheck: provider_check.New(ruleset.NewChecker()),
	})

	oasSrv, err := oas.NewServer(composite, composite, oas.WithErrorHandler(composite.ErrorHandler))
	if err != nil {
		return nil, fmt.Errorf("oas server: %w", err)
	}

	// Routing for every API operation lives in the ogen-generated server (its router maps
	// each path+method to the right operation from the OpenAPI spec). We only delegate the
	// coarse URL buckets it owns — never re-declaring per-operation routes here, so a
	// handler can't be wired to the wrong contract. Auth is enforced inside the ogen
	// server (its SecurityHandler returns 401 for every secured operation); login/logout
	// are the only unauthenticated admin operations.
	r.PathPrefix("/admin/api/").Handler(oasSrv)
	r.PathPrefix("/sub/").Handler(oasSrv)
	r.PathPrefix("/rules/").Handler(oasSrv)
	r.Handle("/healthz", oasSrv)

	mountAdmin(r, cfg, sess, loginHandler)

	return r, nil
}

// mountAdmin mounts the non-API admin surface: static assets, the login page, and the
// SPA shell — the routes the OpenAPI spec doesn't own. The JSON API itself (including
// the login/logout actions) is routed by the ogen server (see buildRouter); these are
// registered after it so /admin/api and /admin/login win over the /admin SPA catch-all.
func mountAdmin(r *mux.Router, cfg config.Config, sess *web.Session, loginHandler *login.Handler) {
	ra := sess.RequireAdmin

	r.PathPrefix("/admin/static/").Handler(web.StaticHandler(cfg.StaticDir))
	r.Handle("/admin/login", loginHandler).Methods(http.MethodGet)

	// SPA shell: serve index.html for /admin and any other admin GET (client-side views).
	shell := ra(func(w http.ResponseWriter, _ *http.Request) { web.ServePage(w, cfg.StaticDir, "index.html") })
	r.PathPrefix("/admin").Methods(http.MethodGet).HandlerFunc(shell)
}
