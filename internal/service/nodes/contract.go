//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package nodes
package nodes

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

// nodeRepo is the nodes repository subset the service needs.
type nodeRepo interface {
	Get(ctx context.Context, id int64) (*entity.Node, error)
	Create(ctx context.Context, n entity.Node) (int64, error)
	Update(ctx context.Context, id int64, n entity.Node, setToken bool) error
	Delete(ctx context.Context, id int64) error
	ConnectionCountsByInbound(ctx context.Context, inboundIDs []int64) (map[int64]int, error)
}

// routingRepo reports how many mihomo rules / proxy-group members reference an inbound.
type routingRepo interface {
	InboundRefCounts(ctx context.Context, inboundIDs []int64) (map[int64]int, error)
}
