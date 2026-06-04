//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package users_api
package users_api

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

type userLister interface {
	List(ctx context.Context) ([]entity.User, error)
}

type fleetReader interface {
	Fleet(ctx context.Context) (*entity.Fleet, error)
}

type connHealth interface {
	MissingConnections(ctx context.Context, u *entity.User) []entity.Connection
}
