//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package user_create
package user_create

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

// creator provisions a new user (the provisioning service satisfies it).
type creator interface {
	CreateUser(ctx context.Context, p entity.UserCreateParams) (*entity.User, error)
}
