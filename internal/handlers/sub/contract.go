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

// EngineRenderer renders one engine's subscription body for a subscriber + config id,
// returning the bytes plus the response metadata (content type, filename) that engine
// needs. The registry (built in the composition root) keys these by Kind; today only
// mihomo is registered.
type EngineRenderer interface {
	Kind() entity.ConfigKind
	Render(ctx context.Context, sub *entity.Subscriber, configID int64) ([]byte, RenderMeta, error)
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
