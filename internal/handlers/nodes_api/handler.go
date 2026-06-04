// Package nodes_api handles GET /admin/api/nodes — the node registry as JSON for
// the admin SPA.
package nodes_api

import (
	"log/slog"
	"net/http"
	"sort"

	"github.com/postlog/subgen/internal/handlers/web"
)

// Handler serves the node registry as JSON.
type Handler struct {
	nodes nodeLister
}

// New builds the handler.
func New(nodes nodeLister) *Handler { return &Handler{nodes: nodes} }

type inboundView struct {
	ID   int64  `json:"id"`   // node_inbounds.id
	Name string `json:"name"` // inbound name (the label is "<node>-<name>")
	Port int    `json:"port"`
}

type row struct {
	ID            int64         `json:"id"`
	Name          string        `json:"name"`
	VPNHost       string        `json:"vpnHost"`
	PanelBaseURL  string        `json:"panelBaseURL"`
	PanelBasePath string        `json:"panelBasePath"`
	Inbounds      []inboundView `json:"inbounds"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.nodes.List(r.Context())
	if err != nil {
		slog.Error("handler nodes_api: list failed", "err", err)
		http.Error(w, "store unavailable", http.StatusInternalServerError)

		return
	}

	rows := make([]row, 0, len(nodes))
	for _, n := range nodes {
		inbounds := make([]inboundView, 0, len(n.Inbounds))
		for _, in := range n.Inbounds {
			inbounds = append(inbounds, inboundView{ID: in.ID, Name: in.Name, Port: in.Port})
		}
		// Deterministic order by name.
		sort.Slice(inbounds, func(i, j int) bool { return inbounds[i].Name < inbounds[j].Name })

		rows = append(rows, row{
			ID: n.ID, Name: n.Name, VPNHost: n.VPNHost,
			PanelBaseURL: n.PanelBaseURL, PanelBasePath: n.PanelBasePath,
			Inbounds: inbounds,
		})
	}

	web.JSON(w, map[string]any{"nodes": rows})
}
