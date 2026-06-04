//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package fleet
package fleet

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

// nodeLister lists the node registry (the nodes repository satisfies it).
type nodeLister interface {
	List(ctx context.Context) ([]entity.Node, error)
}

// panelClient reads inbounds from a 3x-ui panel (the xui client satisfies it).
type panelClient interface {
	ListInbounds(ctx context.Context, t entity.PanelTarget) ([]entity.PanelInbound, error)
}
