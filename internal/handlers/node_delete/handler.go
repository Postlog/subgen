// Package node_delete implements the nodeDelete operation (POST /admin/api/nodes/delete).
// Deletion lives in the nodes service; this handler maps its errors to responses. A node
// whose inbound is still referenced is refused by the database FK (entity.ErrInboundReferenced).
package node_delete

import (
	"context"
	"errors"
	"log/slog"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/oas"
)

// User-facing messages. Exported so apitest can assert against them without duplicating
// the text.
const (
	MsgDeleted           = "Узел удалён"
	MsgNotFound          = "Узел не найден"
	MsgInboundReferenced = "Узел используется — сначала отвяжите его инбаунды от пользователей и правил"
)

// Handler deletes a node via the nodes service.
type Handler struct {
	svc nodesService
}

// New builds the handler.
func New(svc nodesService) *Handler { return &Handler{svc: svc} }

// NodeDelete implements oas.Handler: a still-referenced inbound is a 400, any unexpected
// (store) failure is a 500.
func (h *Handler) NodeDelete(ctx context.Context, req *oas.NodeDeleteReq) (oas.NodeDeleteRes, error) {
	err := h.svc.Delete(ctx, req.ID)
	if err == nil {
		return &oas.MessageResponse{Message: MsgDeleted}, nil
	}

	switch {
	case errors.Is(err, entity.ErrNodeNotFound):
		slog.Warn("handler node_delete: node not found", "id", req.ID)
		return &oas.NodeDeleteBadRequest{ErrMessage: MsgNotFound}, nil
	case errors.Is(err, entity.ErrInboundReferenced):
		slog.Warn("handler node_delete: inbound still referenced", "id", req.ID)
		return &oas.NodeDeleteBadRequest{ErrMessage: MsgInboundReferenced}, nil
	default:
		slog.Error("handler node_delete: delete failed", "id", req.ID, "err", err)
		return nil, err
	}
}
