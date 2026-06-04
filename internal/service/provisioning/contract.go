//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package provisioning
package provisioning

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

// userRepo is the users repository subset provisioning needs.
type userRepo interface {
	Get(ctx context.Context, id int64) (*entity.User, error)
	Create(ctx context.Context, u *entity.User) error
	ReplaceConnections(ctx context.Context, userID int64, inboundIDs []int64) error
	Delete(ctx context.Context, id int64) error
}

// nodeRepo lists the node registry.
type nodeRepo interface {
	List(ctx context.Context) ([]entity.Node, error)
}

// panelClient is the 3x-ui client: stateless, the panel is a per-call target
// (different nodes carry different credentials).
type panelClient interface {
	ListInbounds(ctx context.Context, t entity.PanelTarget) ([]entity.PanelInbound, error)
	AddClient(ctx context.Context, t entity.PanelTarget, inboundIDs []int, cs entity.ClientSpec) error
	DelClient(ctx context.Context, t entity.PanelTarget, email string) error
}

// fleetCache is the narrow cache-invalidation hook (the fleet service satisfies it).
type fleetCache interface {
	Invalidate()
}
