//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package config_api
package config_api

import (
	"context"

	"github.com/postlog/subgen/internal/mihomo"
)

type mihomoReader interface {
	Rules(ctx context.Context) ([]mihomo.RoutingRule, error)
	ProxyGroups(ctx context.Context) ([]mihomo.ProxyGroup, error)
	RuleProviders(ctx context.Context) ([]mihomo.RuleProvider, error)
	Setting(ctx context.Context, key string) (string, error)
}
