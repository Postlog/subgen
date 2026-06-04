//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package nodes_api
package nodes_api

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

type nodeLister interface {
	List(ctx context.Context) ([]entity.Node, error)
}
