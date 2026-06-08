// Package user_recreate implements the userRecreate operation
// (POST /admin/api/users/recreate).
package user_recreate

import (
	"context"

	"github.com/postlog/subgen/internal/handlers/web"
	"github.com/postlog/subgen/internal/oas"
)

// Handler re-provisions a user's panel clients from the store.
type Handler struct {
	svc recreator
}

// New builds the handler.
func New(svc recreator) *Handler { return &Handler{svc: svc} }

// UserRecreate implements oas.Handler.
func (h *Handler) UserRecreate(ctx context.Context, req *oas.UserRecreateReq) (oas.UserRecreateRes, error) {
	if err := h.svc.RecreateUser(ctx, req.ID); err != nil {
		return &oas.UserRecreateBadRequest{ErrMessage: web.UserMessage(err)}, nil
	}

	return &oas.MessageResponse{Message: "Клиенты пересозданы"}, nil
}
