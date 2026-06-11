//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package node_delete
package node_delete

import "context"

// nodeDeleter is the nodes service: it validates the id, refuses a still-referenced node,
// and deletes it (the nodes service satisfies it).
type nodeDeleter interface {
	Delete(ctx context.Context, id int64) error
}
