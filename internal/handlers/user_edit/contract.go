//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package user_edit
package user_edit

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

// editor updates a user's connections and description (the provisioning service
// satisfies it).
type editor interface {
	EditUser(ctx context.Context, p entity.UserEditParams) error
}
