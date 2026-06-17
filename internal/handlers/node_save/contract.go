//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package node_save
package node_save

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

// nodesService is the nodes service: it validates and persists a node (the nodes service
// satisfies it).
type nodesService interface {
	Save(ctx context.Context, n entity.Node) (int64, error)
}
