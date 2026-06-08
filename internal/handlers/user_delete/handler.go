// Package user_delete implements the userDelete operation (POST /admin/api/users/delete).
package user_delete

import (
	"context"

	"github.com/postlog/subgen/internal/handlers/web"
	"github.com/postlog/subgen/internal/oas"
)

// Handler deletes a user and deprovisions its panel clients.
type Handler struct {
	svc deleter
}

// New builds the handler.
func New(svc deleter) *Handler { return &Handler{svc: svc} }

// UserDelete implements oas.Handler.
func (h *Handler) UserDelete(ctx context.Context, req *oas.UserDeleteReq) (oas.UserDeleteRes, error) {
	if err := h.svc.DeleteUser(ctx, req.ID); err != nil {
		return &oas.UserDeleteBadRequest{ErrMessage: web.UserMessage(err)}, nil
	}

	return &oas.MessageResponse{Message: "Пользователь удалён"}, nil
}
