// Command service runs subgen: it serves per-client mihomo (Clash.Meta)
// subscription configs and a small admin panel. Bootstrap settings (listener,
// TLS, secret, admin creds, db path) come from the environment / a local .env
// file; operational config (nodes, rules, rule-providers, base YAML) lives in
// SQLite and is edited via the admin panel.
//
// This file is the composition root: it loads config, opens the repositories,
// constructs the clients/services/handlers and wires the HTTP router (the ogen server
// + a static-asset handler alongside it).
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

	"github.com/postlog/subgen/internal/cert"
	"github.com/postlog/subgen/internal/clients/xui"
	"github.com/postlog/subgen/internal/config"
	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/handlers/admin_shell"
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
	nodessvc "github.com/postlog/subgen/internal/service/nodes"
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

// buildRouter wires the HTTP handler. Every typed route — the API, the subscription /
// rules endpoints, and the browser pages (login page + SPA shell) — is owned by the
// ogen-generated server, whose router multiplexes them from the OpenAPI spec; it is the
// root handler. The only thing mounted alongside it is the static-asset tree
// (/admin/static/*), a plain filesystem handler — not a typed API — as ogen's
// static-router guidance recommends. No third-party mux.
func buildRouter(cfg config.Config, usersRepo *users.Repository, nodesRepo *nodes.Repository, routingRepo *routing.Repository, configsRepo *configs.Repository, fleetSvc *fleet.Service, prov *provisioning.Service, mirror *ruleset.Mirror) (http.Handler, error) {
	// Per-engine subscription renderers, keyed by config kind. Adding an engine = a
	// new engineRenderer + one entry here; the route and handler don't change.
	renderers := map[entity.ConfigKind]sub.EngineRenderer{
		entity.ConfigKindMihomo: sub.NewMihomoRenderer(routingRepo, cfg.PublicBase),
	}

	sess := web.NewSession(cfg.Secret)

	nodesSvc := nodessvc.New(nodesRepo, routingRepo)

	// The login handler serves both the sign-in action (POST /admin/api/login) and the
	// login PAGE (GET /admin/login).
	loginHandler := login.New(sess, cfg.AdminUser, cfg.AdminPassword, cfg.StaticDir)

	// The ogen server (api composite) — forwards every operation to its per-action
	// handler, with the admin session cookie + idiomatic errors handled inside.
	composite := api.New(sess, api.Handlers{
		Healthz: healthz.New(),
		Sub: sub.New(
			usersRepo, fleetSvc, configsRepo, renderers, cfg.Secret,
		),
		Rules:      rules.New(mirror),
		Login:      loginHandler,
		Logout:     logout.New(sess),
		AdminShell: admin_shell.New(sess, cfg.StaticDir),

		UsersGet:     users_get.New(usersRepo, fleetSvc, cfg.Secret, cfg.PublicBase),
		UserCreate:   user_create.New(prov),
		UserEdit:     user_edit.New(prov),
		UserDelete:   user_delete.New(prov),
		UserRecreate: user_recreate.New(prov),
		NodesGet:     nodes_get.New(nodesRepo),
		NodeSave:     node_save.New(nodesSvc),
		NodeDelete:   node_delete.New(nodesSvc),

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

	// The ogen server is the root handler; the static assets are the one route served
	// outside it (http.ServeMux picks the longest prefix, so /admin/static/* goes to the
	// filesystem handler and everything else to ogen).
	mux := http.NewServeMux()
	mux.Handle("/admin/static/", web.StaticHandler(cfg.StaticDir))
	mux.Handle("/", oasSrv)

	return mux, nil
}
