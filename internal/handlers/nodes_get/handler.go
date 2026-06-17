// Package nodes_get implements the nodesGet operation (GET /admin/api/nodes) — the
// node registry for the admin SPA.
package nodes_get

import (
	"context"
	"log/slog"
	"sort"

	"github.com/postlog/subgen/internal/oas"
)

// Handler serves the node registry.
type Handler struct {
	nodes nodesRepo
}

// New builds the handler.
func New(nodes nodesRepo) *Handler { return &Handler{nodes: nodes} }

// NodesGet implements oas.Handler: it lists the fleet nodes with their inbounds.
func (h *Handler) NodesGet(ctx context.Context) (oas.NodesGetRes, error) {
	nodes, err := h.nodes.List(ctx)
	if err != nil {
		slog.Error("handler nodes_get: list nodes failed", "err", err)
		return nil, err
	}

	items := make([]oas.NodesGetOKNodesItem, 0, len(nodes))

	for _, n := range nodes {
		inbounds := make([]oas.NodesGetOKNodesItemInboundsItem, 0, len(n.Inbounds))
		for _, in := range n.Inbounds {
			inbounds = append(inbounds, oas.NodesGetOKNodesItemInboundsItem{ID: in.ID, Name: in.Name, Port: in.Port})
		}
		// Deterministic order by name.
		sort.Slice(inbounds, func(i, j int) bool { return inbounds[i].Name < inbounds[j].Name })

		items = append(items, oas.NodesGetOKNodesItem{
			ID: n.ID, Name: n.Name, VpnHost: n.VPNHost,
			PanelBaseURL: n.PanelBaseURL, PanelBasePath: n.PanelBasePath, Inbounds: inbounds,
		})
	}

	return &oas.NodesGetOK{Nodes: items}, nil
}
