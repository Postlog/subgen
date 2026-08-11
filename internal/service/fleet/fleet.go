// Package fleet assembles the normalized fleet (subscribers + their proxies) from live
// panel snapshots, fresh on every call — there is no cache, so the admin health badge
// and live subscriptions always reflect the panels right now. One unreachable panel is
// tolerated (its node is skipped); only a total outage (every panel failing) errors.
package fleet

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/postlog/subgen/internal/entity"
)

// Service fetches every node's inbounds and assembles the fleet.
type Service struct {
	client panelClient
	nodes  nodesRepo
	conns  connsRepo
}

// New builds the fleet service.
func New(client panelClient, nodes nodesRepo, conns connsRepo) *Service {
	return &Service{client: client, nodes: nodes, conns: conns}
}

// Fleet lists every node's inbounds and assembles the fleet, fresh on every call.
func (s *Service) Fleet(ctx context.Context) (*entity.Fleet, error) {
	return s.fetch(ctx)
}

// fetch lists every node's inbounds and builds the fleet. One unreachable panel is
// tolerated (its node is skipped) so clients on healthy nodes keep working — only a
// total outage (every panel failing) returns an error.
func (s *Service) fetch(ctx context.Context) (*entity.Fleet, error) {
	nodes, err := s.nodes.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("nodes.List: %w", err)
	}

	var snaps []panelSnapshot

	var errs []string

	for _, n := range nodes {
		inbs, err := s.client.ListInbounds(ctx, target(n))
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", n.Name, err))
			// Background-path visibility: this is swallowed (partial outage is
			// tolerated), so it would be invisible if not logged here.
			slog.Warn("service fleet: panel unreachable, skipping", "node", n.Name, "err", err)

			continue
		}

		snaps = append(snaps, panelSnapshot{node: n, inbounds: inbs})
	}

	if len(snaps) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("all panels failed: %s", strings.Join(errs, "; "))
	}

	fleet := buildFleet(snaps)

	// hysteria2 inbounds aren't on 3x-ui (Xray has no hysteria2), so their proxies come
	// from the DB, not the panel: subscribers from user_connections (same access model as
	// VLESS — the admin decides), params from the inbound's stored creds.
	subs, err := s.conns.InboundSubscribers(ctx)
	if err != nil {
		return nil, fmt.Errorf("conns.InboundSubscribers: %w", err)
	}

	addHysteria2Inbounds(fleet, nodes, subs)

	return fleet, nil
}

func target(n entity.Node) entity.PanelTarget {
	return entity.PanelTarget{BaseURL: n.PanelBaseURL, BasePath: n.PanelBasePath, Token: n.Token}
}
