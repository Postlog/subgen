//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package config_api
package config_api

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/mihomo"
)

// configResolver resolves the config scope (base vs a user's custom) to a config id.
type configResolver interface {
	BaseConfigID(ctx context.Context, kind entity.ConfigKind) (int64, bool, error)
	UserConfigID(ctx context.Context, userID int64, kind entity.ConfigKind) (int64, bool, error)
}

// mihomoReader reads one config's mihomo content (scoped by config id).
type mihomoReader interface {
	Rules(ctx context.Context, configID int64) ([]mihomo.RoutingRule, error)
	ProxyGroups(ctx context.Context, configID int64) ([]mihomo.ProxyGroup, error)
	RuleProviders(ctx context.Context, configID int64) ([]mihomo.RuleProvider, error)
	Setting(ctx context.Context, configID int64, key string) (string, error)
}
