//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package config_save
package config_save

import (
	"context"

	"github.com/postlog/subgen/internal/mihomo"
)

// mihomoSaver persists the operator-edited mihomo config (rules + proxy-groups +
// providers + base YAML) atomically.
type mihomoSaver interface {
	SaveMihomoConfig(ctx context.Context, rules []mihomo.RoutingRule, groups []mihomo.ProxyGroup, rps []mihomo.RuleProvider, baseYAML string) error
}
