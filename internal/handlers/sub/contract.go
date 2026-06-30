//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package sub
package sub

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/mihomo"
)

// usersRepo lists the subscription IDs of service-owned users (for token
// reverse-lookup) and resolves a matched sub_id to its user id.
type usersRepo interface {
	SubIDs(ctx context.Context) ([]string, error)
	IDBySubID(ctx context.Context, subID string) (int64, error)
}

// fleetService returns the current (cached) fleet snapshot.
type fleetService interface {
	Fleet(ctx context.Context) (*entity.Fleet, error)
}

// configsRepo picks the config to render for a subscriber: a user's custom config
// when present, otherwise the engine's base.
type configsRepo interface {
	UserConfigID(ctx context.Context, userID int64, kind entity.ConfigKind) (int64, bool, error)
	BaseConfigID(ctx context.Context, kind entity.ConfigKind) (int64, bool, error)
}

// EngineRenderer renders one engine's subscription artifacts for a subscriber + config id.
// Render returns the main config body plus its response metadata; token is the subscriber's
// subscription token, which the engine weaves into the per-token provider URLs it emits.
// RenderProxies returns the node-list payload served at /sub/{kind}/{token}/proxies.
// RenderRuleProvider returns an authored rule-provider's body (served at .../rules/{name})
// and whether such a provider exists. The registry (built in the composition root) keys
// these by Kind; today only mihomo is registered.
type EngineRenderer interface {
	Kind() entity.ConfigKind
	Render(ctx context.Context, sub *entity.Subscriber, configID int64, token string) ([]byte, RenderMeta, error)
	RenderProxies(ctx context.Context, sub *entity.Subscriber) ([]byte, error)
	RenderRuleProvider(ctx context.Context, configID int64, name string) ([]byte, bool, error)
}

// routingRepo reads one config's mihomo content (scoped by config id) for the mihomo
// renderer.
type routingRepo interface {
	Rules(ctx context.Context, configID int64) ([]mihomo.RoutingRule, error)
	ProxyGroups(ctx context.Context, configID int64) ([]mihomo.ProxyGroup, error)
	RuleProviders(ctx context.Context, configID int64) ([]mihomo.RuleProvider, error)
	Setting(ctx context.Context, configID int64, key string) (string, error)
	Profile(ctx context.Context, configID int64) (mihomo.Profile, error)
}
