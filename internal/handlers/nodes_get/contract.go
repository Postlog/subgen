//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package nodes_get
package nodes_get

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

type nodeLister interface {
	List(ctx context.Context) ([]entity.Node, error)
}
