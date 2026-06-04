// Package fleet builds the normalized fleet (subscribers + their proxies) from
// live panel snapshots and serves it behind a TTL cache with stale-on-error, so a
// brief panel hiccup never breaks live subscriptions. The cache is narrow — it
// wraps just this service, not a global snapshot of everything.
package fleet

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/postlog/subgen/internal/entity"
)

// Service fetches every node's inbounds and assembles the fleet.
type Service struct {
	client panelClient
	nodes  nodeLister
	ttl    time.Duration

	mu      sync.RWMutex
	fleet   *entity.Fleet
	at      time.Time
	refresh sync.Mutex
}

// New builds the fleet service. A non-positive ttl defaults to 5m.
func New(client panelClient, nodes nodeLister, ttl time.Duration) *Service {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	return &Service{client: client, nodes: nodes, ttl: ttl}
}

// Invalidate forces the next Fleet call to refetch (call after a write to a panel
// or the node registry).
func (s *Service) Invalidate() {
	s.mu.Lock()
	s.at = time.Time{}
	s.mu.Unlock()
}

// Fleet returns a fleet no older than ttl, refreshing if needed. Only one refresh
// runs at a time; on refresh error it returns the last good fleet when one exists.
func (s *Service) Fleet(ctx context.Context) (*entity.Fleet, error) {
	s.mu.RLock()
	f, at := s.fleet, s.at
	s.mu.RUnlock()

	if f != nil && time.Since(at) < s.ttl {
		return f, nil
	}

	s.refresh.Lock()
	defer s.refresh.Unlock()

	// Re-check: another goroutine may have refreshed while we waited.
	s.mu.RLock()
	f, at = s.fleet, s.at
	s.mu.RUnlock()

	if f != nil && time.Since(at) < s.ttl {
		return f, nil
	}

	nf, err := s.fetch(ctx)
	if err != nil {
		if f != nil {
			return f, nil // stale-on-error
		}

		return nil, err
	}

	s.mu.Lock()
	s.fleet, s.at = nf, time.Now()
	s.mu.Unlock()

	return nf, nil
}

// fetch lists every node's inbounds and builds the fleet. One unreachable panel is
// tolerated (its node is skipped) so clients on healthy nodes keep working — only
// a total outage (every panel failing) returns an error.
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

	return buildFleet(snaps), nil
}

func target(n entity.Node) entity.PanelTarget {
	return entity.PanelTarget{BaseURL: n.PanelBaseURL, BasePath: n.PanelBasePath, Token: n.Token}
}
