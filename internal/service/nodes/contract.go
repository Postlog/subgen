//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package nodes
package nodes

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

// nodesRepo is the nodes repository subset the service needs. Referential integrity (an
// inbound still referenced on update/delete) is enforced by the database FK and returned
// as entity.ErrInboundReferenced — the service does not pre-check it.
type nodesRepo interface {
	Create(ctx context.Context, n entity.Node) (int64, error)
	Update(ctx context.Context, id int64, n entity.Node, setToken bool) error
	Delete(ctx context.Context, id int64) error
}
