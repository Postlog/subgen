// Package node_delete implements the nodeDelete operation (POST /admin/api/nodes/delete).
// Id validation and the FK-block check live in the nodes service; this handler maps its
// errors to responses.
package node_delete

import (
	"context"
	"errors"
	"log/slog"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/handlers/web"
	"github.com/postlog/subgen/internal/oas"
)

const msgDeleted = "Узел удалён"

// Handler deletes a node via the nodes service.
type Handler struct {
	svc nodeDeleter
}

// New builds the handler.
func New(svc nodeDeleter) *Handler { return &Handler{svc: svc} }

// NodeDelete implements oas.Handler: an invalid id or a still-referenced inbound is a 400,
// any unexpected (store) failure is a 500.
func (h *Handler) NodeDelete(ctx context.Context, req *oas.NodeDeleteReq) (oas.NodeDeleteRes, error) {
	err := h.svc.Delete(ctx, req.ID)
	if err == nil {
		return &oas.MessageResponse{Message: msgDeleted}, nil
	}

	var blocked entity.InboundsBlockedError

	switch {
	case errors.As(err, &blocked):
		slog.Warn("handler node_delete: inbound still referenced", "id", req.ID)
		return &oas.NodeDeleteBadRequest{ErrMessage: web.InboundsBlockedMessage(blocked)}, nil
	default:
		slog.Error("handler node_delete: delete failed", "id", req.ID, "err", err)
		return nil, err
	}
}
