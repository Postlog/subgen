// Command subctl is the subgen CLI utility: it inspects the fleet and renders
// subscriptions to stdout (debugging). It reads the same .env + store as the
// service but does not serve HTTP.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/postlog/subgen/internal/clients/xui"
	"github.com/postlog/subgen/internal/config"
	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/mihomo/render"
	"github.com/postlog/subgen/internal/repository"
	"github.com/postlog/subgen/internal/repository/nodes"
	"github.com/postlog/subgen/internal/repository/routing"
	"github.com/postlog/subgen/internal/service/fleet"
	"github.com/postlog/subgen/internal/token"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	envFile := flag.String("env", ".env", "path to .env file (KEY=VALUE); the process environment takes precedence")
	dumpFleet := flag.Bool("dump-fleet", false, "fetch the fleet, print subId/token/proxies, and exit")
	printSub := flag.String("print", "", "render the config for a given subId to stdout and exit")

	flag.Parse()

	if !*dumpFleet && *printSub == "" {
		return errors.New("nothing to do: pass -dump-fleet or -print <subId>")
	}

	cfg, err := config.Load(*envFile)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := repository.Open(ctx, cfg.DBPath)
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}

	defer db.Close()

	nodesRepo := nodes.New(db)
	routingRepo := routing.New(db)

	fl, err := fleet.New(xui.New(), nodesRepo, cfg.CacheTTL).Fleet(ctx)
	if err != nil {
		return fmt.Errorf("fetch fleet: %w", err)
	}

	if *printSub != "" {
		return printOne(ctx, cfg, routingRepo, fl, *printSub)
	}

	base := strings.TrimRight(cfg.PublicBase, "/")

	for _, id := range fl.SubIDs() {
		sub := fl.Sub(id)
		tok := token.Make(cfg.Secret, id)
		fmt.Printf("subId=%s emails=%v proxies=%d\n", id, sub.Emails, len(sub.Proxies))

		if base != "" {
			fmt.Printf("    url: %s/sub/%s\n", base, tok)
		}

		for _, p := range sub.Proxies {
			fmt.Printf("    - %-18s %s:%d %s/%s\n", p.Name, p.Server, p.Port, p.Network, p.Security)
		}
	}

	return nil
}

// printOne renders one subscriber's config to stdout.
func printOne(ctx context.Context, cfg config.Config, routingRepo *routing.Repository, fl *entity.Fleet, subID string) error {
	sub := fl.Sub(subID)
	if sub == nil {
		return fmt.Errorf("subId %q not found", subID)
	}

	opts, err := renderOptions(ctx, cfg, routingRepo)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	out, err := render.Render(sub, opts)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}

	_, _ = os.Stdout.Write(out)

	return nil
}

func renderOptions(ctx context.Context, cfg config.Config, routingRepo *routing.Repository) (render.Options, error) {
	rules, err := routingRepo.Rules(ctx)
	if err != nil {
		return render.Options{}, err
	}

	groups, err := routingRepo.ProxyGroups(ctx)
	if err != nil {
		return render.Options{}, err
	}

	provs, err := routingRepo.RuleProviders(ctx)
	if err != nil {
		return render.Options{}, err
	}

	base, err := routingRepo.Setting(ctx, "base_yaml")
	if err != nil {
		return render.Options{}, err
	}

	return render.Options{
		BaseYAML:   base,
		Rules:      rules,
		Groups:     groups,
		Providers:  provs,
		PublicBase: cfg.PublicBase,
	}, nil
}
