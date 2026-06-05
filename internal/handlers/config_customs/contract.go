//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package config_customs
package config_customs

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

// configLister lists the user ids that have a custom config for an engine.
type configLister interface {
	UserConfigUserIDs(ctx context.Context, kind entity.ConfigKind) ([]int64, error)
}

// userLister lists the users (for resolving ids to display names).
type userLister interface {
	List(ctx context.Context) ([]entity.User, error)
}
