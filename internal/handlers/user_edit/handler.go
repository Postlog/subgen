// Package user_edit implements the userEdit operation (POST /admin/api/users/edit).
package user_edit

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/handlers/web"
	"github.com/postlog/subgen/internal/oas"
)

// Handler re-binds a user to a new inbound set.
type Handler struct {
	svc editor
}

// New builds the handler.
func New(svc editor) *Handler { return &Handler{svc: svc} }

// UserEdit implements oas.Handler.
func (h *Handler) UserEdit(ctx context.Context, req *oas.UserEditReq) (oas.UserEditRes, error) {
	if err := h.svc.EditUser(ctx, req.ID, entity.ConnectionSelection{InboundIDs: req.InboundIDs}); err != nil {
		return &oas.UserEditBadRequest{ErrMessage: web.UserMessage(err)}, nil
	}

	return &oas.MessageResponse{Message: "Подключения обновлены"}, nil
}
