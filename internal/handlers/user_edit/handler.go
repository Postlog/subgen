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
	MsgUpdated         = "Connections updated"
	MsgNoConnection    = "Select at least one connection"
	MsgInboundNotFound = "The specified inbound was not found"
	MsgDescTooLong     = "Description is too long (max 500 characters)"
)

// Handler re-binds a user to a new inbound set.
type Handler struct {
	svc provisioningService
}

// New builds the handler.
func New(svc provisioningService) *Handler { return &Handler{svc: svc} }

// UserEdit implements oas.Handler: invalid input is a 400, any unexpected (infra)
// failure is a 500.
func (h *Handler) UserEdit(ctx context.Context, req *oas.UserEditReq) (oas.UserEditRes, error) {
	var desc *string
	if v, ok := req.Description.Get(); ok {
		desc = &v
	}

	err := h.svc.EditUser(ctx, entity.UserEditParams{
		ID:          req.ID,
		Description: desc,
		InboundIDs:  req.InboundIDs,
	})
	if err == nil {
		return &oas.MessageResponse{Message: MsgUpdated}, nil
	}

	switch {
	case errors.Is(err, entity.ErrNoConnectionSelected):
		slog.Warn("handler user_edit: no connection selected", "id", req.ID)
		return &oas.UserEditBadRequest{ErrMessage: MsgNoConnection}, nil
	case errors.Is(err, entity.ErrDescriptionTooLong):
		slog.Warn("handler user_edit: description too long", "id", req.ID)
		return &oas.UserEditBadRequest{ErrMessage: MsgDescTooLong}, nil
	case errors.Is(err, entity.ErrInboundNotFound):
		slog.Warn("handler user_edit: inbound not found", "id", req.ID)
		return &oas.UserEditBadRequest{ErrMessage: MsgInboundNotFound}, nil
	default:
		slog.Error("handler user_edit: edit failed", "id", req.ID, "err", err)
		return nil, err
	}
}
