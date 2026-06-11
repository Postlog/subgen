// Package user_edit implements the userEdit operation (POST /admin/api/users/edit).
package user_edit

import (
	"context"
	"errors"
	"log/slog"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/oas"
)

// User-facing messages for the domain errors EditUser can return.
const (
	msgNoConnection    = "Выберите хотя бы одно подключение"
	msgInboundNotFound = "Указанный инбаунд не найден"
	msgInvalidID       = "Неверный идентификатор" // moved-from-schema minimum:1 guard
)

// Handler re-binds a user to a new inbound set.
type Handler struct {
	svc editor
}

// New builds the handler.
func New(svc editor) *Handler { return &Handler{svc: svc} }

// UserEdit implements oas.Handler: invalid input is a 400, any unexpected (infra)
// failure is a 500.
func (h *Handler) UserEdit(ctx context.Context, req *oas.UserEditReq) (oas.UserEditRes, error) {
	if req.ID < 1 {
		slog.Warn("handler user_edit: invalid id", "id", req.ID)
		return &oas.UserEditBadRequest{ErrMessage: msgInvalidID}, nil
	}

	err := h.svc.EditUser(ctx, req.ID, entity.ConnectionSelection{InboundIDs: req.InboundIDs})
	if err == nil {
		return &oas.MessageResponse{Message: "Подключения обновлены"}, nil
	}

	switch {
	case errors.Is(err, entity.ErrNoConnectionSelected):
		slog.Warn("handler user_edit: no connection selected", "id", req.ID)
		return &oas.UserEditBadRequest{ErrMessage: msgNoConnection}, nil
	case errors.Is(err, entity.ErrInboundNotFound):
		slog.Warn("handler user_edit: inbound not found", "id", req.ID)
		return &oas.UserEditBadRequest{ErrMessage: msgInboundNotFound}, nil
	default:
		slog.Error("handler user_edit: edit failed", "id", req.ID, "err", err)
		return nil, err
	}
}
