// Package node_delete handles POST /admin/nodes/delete.
package node_delete

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/postlog/subgen/internal/handlers/web"
)

const msgDeleted = "Узел удалён"

// Handler deletes a node, refusing if any inbound is still referenced (by a user
// connection or a mihomo rule / proxy-group member).
type Handler struct {
	nodes   nodeRepo
	routing routingRepo
}

// New builds the handler.
func New(nodes nodeRepo, routing routingRepo) *Handler {
	return &Handler{nodes: nodes, routing: routing}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID int64 `json:"id"`
	}

	if err := web.DecodeJSON(r, &req); err != nil {
		slog.Warn("handler node_delete: decode failed", "err", err)
		web.WriteJSON(w, false, web.MsgBadRequest)

		return
	}

	id := req.ID

	msg, err := web.InboundsBlocking(r.Context(), h.nodes, h.routing, id, nil)
	if err == nil && msg != "" {
		err = fmt.Errorf("%s", msg)
	}

	if err == nil {
		err = h.nodes.Delete(r.Context(), id)
	}

	if err != nil {
		slog.Warn("handler node_delete: delete failed", "id", id, "err", err)
	}

	web.JSONResult(w, msgDeleted, err)
}
