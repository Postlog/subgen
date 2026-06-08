//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package users_api
package users_api

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

type userLister interface {
	ListPage(ctx context.Context, p entity.UserListParams) (entity.UserPage, error)
}

type fleetReader interface {
	Fleet(ctx context.Context) (*entity.Fleet, error)
}
