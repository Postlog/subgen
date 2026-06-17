//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package users_get
package users_get

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

type subLinker interface {
	Links(ctx context.Context, users []entity.User) (map[int64][]entity.SubLink, error)
}
