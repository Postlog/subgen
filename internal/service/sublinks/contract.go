//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package sublinks
package sublinks

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/mihomo"
)

// configResolver resolves which config a user effectively gets for an engine — a custom
// config when present, else the base — so the deeplink name can carry that config's
// profile title.
type configResolver interface {
	BaseConfigID(ctx context.Context, kind entity.ConfigKind) (int64, bool, error)
	UserConfigID(ctx context.Context, userID int64, kind entity.ConfigKind) (int64, bool, error)
	UserConfigUserIDs(ctx context.Context, kind entity.ConfigKind) ([]int64, error)
}

// profileReader reads a config's subscription-profile knobs; only the title is used here
// (the name an app deeplink labels the imported profile with).
type profileReader interface {
	Profile(ctx context.Context, configID int64) (mihomo.Profile, error)
}
