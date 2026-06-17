//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package node_delete
package node_delete

import "context"

// nodesService is the nodes service: it deletes a node (the nodes service satisfies it). A
// node whose inbound is still referenced is refused by the database FK, returned as
// entity.ErrInboundReferenced.
type nodesService interface {
	Delete(ctx context.Context, id int64) error
}
