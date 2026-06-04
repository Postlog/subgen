//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package sub
package sub

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/mihomo"
)

// subIDLister lists the subscription IDs of service-owned users.
type subIDLister interface {
	SubIDs(ctx context.Context) ([]string, error)
}

// fleetReader returns the current (cached) fleet snapshot.
type fleetReader interface {
	Fleet(ctx context.Context) (*entity.Fleet, error)
}

// mihomoReader reads the operator-edited mihomo config from the store (routing
// rules, proxy-groups, rule-providers and the base YAML).
type mihomoReader interface {
	Rules(ctx context.Context) ([]mihomo.RoutingRule, error)
	ProxyGroups(ctx context.Context) ([]mihomo.ProxyGroup, error)
	RuleProviders(ctx context.Context) ([]mihomo.RuleProvider, error)
	Setting(ctx context.Context, key string) (string, error)
}
