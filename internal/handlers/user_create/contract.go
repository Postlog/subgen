//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package user_create
package user_create

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

// provisioningService provisions a new user (the provisioning service satisfies it).
type provisioningService interface {
	CreateUser(ctx context.Context, p entity.UserCreateParams) (*entity.User, error)
}
