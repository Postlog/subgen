//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package config_save
package config_save

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/mihomo"
)

// configResolver resolves the save scope to a config id: the base (created on first
// save) or a user's existing custom config.
type configResolver interface {
	EnsureBaseConfigID(ctx context.Context, kind entity.ConfigKind) (int64, error)
	UserConfigID(ctx context.Context, userID int64, kind entity.ConfigKind) (int64, bool, error)
}

// mihomoSaver persists one config's mihomo content (rules + proxy-groups + providers
// + base YAML) atomically, scoped by config id.
type mihomoSaver interface {
	SaveMihomoConfig(ctx context.Context, configID int64, rules []mihomo.RoutingRule, groups []mihomo.ProxyGroup, rps []mihomo.RuleProvider, baseYAML string) error
}
