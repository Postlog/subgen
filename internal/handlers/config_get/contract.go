//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package config_get
package config_get

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/mihomo"
)

// configsRepo resolves the config scope (base vs a user's custom) to a config id.
type configsRepo interface {
	BaseConfigID(ctx context.Context, kind entity.ConfigKind) (int64, bool, error)
	UserConfigID(ctx context.Context, userID int64, kind entity.ConfigKind) (int64, bool, error)
}

// routingRepo reads one config's mihomo content (scoped by config id).
type routingRepo interface {
	Rules(ctx context.Context, configID int64) ([]mihomo.RoutingRule, error)
	ProxyGroups(ctx context.Context, configID int64) ([]mihomo.ProxyGroup, error)
	RuleProviders(ctx context.Context, configID int64) ([]mihomo.RuleProvider, error)
	Setting(ctx context.Context, configID int64, key string) (string, error)
	Profile(ctx context.Context, configID int64) (mihomo.Profile, error)
}
