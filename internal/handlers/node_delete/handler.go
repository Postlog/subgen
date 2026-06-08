// Package node_delete implements the nodeDelete operation (POST /admin/api/nodes/delete).
package node_delete

import (
	"context"
	"log/slog"

	"github.com/postlog/subgen/internal/handlers/web"
	"github.com/postlog/subgen/internal/oas"
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

// NodeDelete implements oas.Handler: it deletes the node, returning 400 when a removed
// inbound is still referenced.
func (h *Handler) NodeDelete(ctx context.Context, req *oas.NodeDeleteReq) (oas.NodeDeleteRes, error) {
	msg, err := web.InboundsBlocking(ctx, h.nodes, h.routing, req.ID, nil)
	if err != nil {
		slog.Error("handler node_delete: inbound-block check failed", "id", req.ID, "err", err)
		return nil, err
	}

	if msg != "" {
		slog.Warn("handler node_delete: inbound still referenced", "id", req.ID)
		return &oas.NodeDeleteBadRequest{ErrMessage: msg}, nil
	}

	if err := h.nodes.Delete(ctx, req.ID); err != nil {
		slog.Error("handler node_delete: delete failed", "id", req.ID, "err", err)
		return nil, err
	}

	return &oas.MessageResponse{Message: msgDeleted}, nil
}
