//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package user_edit
package user_edit

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

// editor updates a user's connections (the provisioning service satisfies it).
type editor interface {
	EditUser(ctx context.Context, id int64, sel entity.ConnectionSelection) error
}
