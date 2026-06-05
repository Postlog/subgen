//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package custom_delete
package custom_delete

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

// configDeleter removes a user's custom config (its content cascades away).
type configDeleter interface {
	DeleteUserConfig(ctx context.Context, userID int64, kind entity.ConfigKind) error
}
