//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package custom_create
package custom_create

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

// configCreator creates a user's custom config as a snapshot of the engine's base.
type configCreator interface {
	CreateUserConfig(ctx context.Context, userID int64, kind entity.ConfigKind) (int64, error)
}
