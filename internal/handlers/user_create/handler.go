// Package user_create implements the userCreate operation (POST /admin/api/users/create).
package user_create

import (
	"context"
	"errors"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/handlers/web"
	"github.com/postlog/subgen/internal/oas"
)

// Handler provisions a new user.
type Handler struct {
	svc creator
}

// New builds the handler.
func New(svc creator) *Handler { return &Handler{svc: svc} }

// UserCreate implements oas.Handler: a nickname clash (or a panel client clash) is a
// 409; other invalid input is a 400.
func (h *Handler) UserCreate(ctx context.Context, req *oas.UserCreateReq) (oas.UserCreateRes, error) {
	_, err := h.svc.CreateUser(ctx, req.Name, entity.ConnectionSelection{InboundIDs: req.InboundIDs})
	if err == nil {
		return &oas.MessageResponse{Message: "Создан пользователь"}, nil
	}

	var pce entity.PanelClientExistsError
	if errors.Is(err, entity.ErrNameTaken) || errors.As(err, &pce) {
		return &oas.UserCreateConflict{ErrMessage: web.UserMessage(err)}, nil
	}

	return &oas.UserCreateBadRequest{ErrMessage: web.UserMessage(err)}, nil
}
