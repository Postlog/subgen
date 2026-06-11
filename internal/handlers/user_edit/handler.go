// Package user_edit implements the userEdit operation (POST /admin/api/users/edit).
package user_edit

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
	MsgUpdated         = "Подключения обновлены"
	MsgNoConnection    = "Выберите хотя бы одно подключение"
	MsgInboundNotFound = "Указанный инбаунд не найден"
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
	err := h.svc.EditUser(ctx, req.ID, entity.ConnectionSelection{InboundIDs: req.InboundIDs})
	if err == nil {
		return &oas.MessageResponse{Message: MsgUpdated}, nil
	}

	switch {
	case errors.Is(err, entity.ErrNoConnectionSelected):
		slog.Warn("handler user_edit: no connection selected", "id", req.ID)
		return &oas.UserEditBadRequest{ErrMessage: MsgNoConnection}, nil
	case errors.Is(err, entity.ErrInboundNotFound):
		slog.Warn("handler user_edit: inbound not found", "id", req.ID)
		return &oas.UserEditBadRequest{ErrMessage: MsgInboundNotFound}, nil
	default:
		slog.Error("handler user_edit: edit failed", "id", req.ID, "err", err)
		return nil, err
	}
}
