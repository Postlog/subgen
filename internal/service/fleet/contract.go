//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package fleet
package fleet

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

// nodesRepo lists the node registry (the nodes repository satisfies it).
type nodesRepo interface {
	List(ctx context.Context) ([]entity.Node, error)
}

// panelClient reads inbounds from a 3x-ui panel (the xui client satisfies it).
type panelClient interface {
	ListInbounds(ctx context.Context, t entity.PanelTarget) ([]entity.PanelInbound, error)
}

// connsRepo lists per-inbound subscribers (sub_ids) — the source for STATIC (hysteria2)
// inbounds, which are not on any panel (the users repository satisfies it).
type connsRepo interface {
	InboundSubscribers(ctx context.Context) (map[int64][]string, error)
}
