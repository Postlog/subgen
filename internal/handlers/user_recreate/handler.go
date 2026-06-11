// Package user_recreate implements the userRecreate operation
// (POST /admin/api/users/recreate).
package user_recreate

import (
	"context"
	"log/slog"

	"github.com/postlog/subgen/internal/oas"
)

// msgInvalidID is returned for a non-positive id (the moved-from-schema minimum:1 guard).
const msgInvalidID = "Неверный идентификатор"

// Handler re-provisions a user's panel clients from the store.
type Handler struct {
	svc recreator
}

// New builds the handler.
func New(svc recreator) *Handler { return &Handler{svc: svc} }

// UserRecreate implements oas.Handler. RecreateUser surfaces no domain (4xx) error — a
// missing user or a panel/store failure is an internal condition — so any failure is a
// logged 500.
func (h *Handler) UserRecreate(ctx context.Context, req *oas.UserRecreateReq) (oas.UserRecreateRes, error) {
	if req.ID < 1 {
		slog.Warn("handler user_recreate: invalid id", "id", req.ID)
		return &oas.UserRecreateBadRequest{ErrMessage: msgInvalidID}, nil
	}

	if err := h.svc.RecreateUser(ctx, req.ID); err != nil {
		slog.Error("handler user_recreate: recreate failed", "id", req.ID, "err", err)
		return nil, err
	}

	return &oas.MessageResponse{Message: "Клиенты пересозданы"}, nil
}
