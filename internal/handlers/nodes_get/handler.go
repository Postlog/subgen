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
			item := oas.NodesGetOKNodesItemInboundsItem{ID: in.ID, Name: in.Name, Port: in.Port}
			if in.Kind != "" {
				item.Kind = oas.NewOptString(in.Kind)
			}
			// Return the non-secret hysteria2 params so the edit form prefills them; the
			// password is write-only and deliberately never sent back.
			if in.IsHysteria2() && in.Hysteria2 != nil {
				item.Hysteria2 = oas.NewOptNodesGetOKNodesItemInboundsItemHysteria2(oas.NodesGetOKNodesItemInboundsItemHysteria2{
					Obfs: oas.NewOptString(in.Hysteria2.Obfs),
					Sni:  oas.NewOptString(in.Hysteria2.SNI),
					Up:   oas.NewOptString(in.Hysteria2.Up),
					Down: oas.NewOptString(in.Hysteria2.Down),
				})
			}

			inbounds = append(inbounds, item)
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
