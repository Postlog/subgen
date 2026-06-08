//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package node_delete
package node_delete

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

// nodeRepo is the nodes repository (also satisfies web.NodeConnChecker).
type nodeRepo interface {
	Get(ctx context.Context, id int64) (*entity.Node, error)
	Delete(ctx context.Context, id int64) error
	ConnectionCountsByInbound(ctx context.Context, inboundIDs []int64) (map[int64]int, error)
}

// routingRepo reports how many mihomo rules / proxy-group members reference an
// inbound (satisfies web.InboundRefChecker).
type routingRepo interface {
	InboundRefCounts(ctx context.Context, inboundIDs []int64) (map[int64]int, error)
}
