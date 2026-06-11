// Package user_delete implements the userDelete operation (POST /admin/api/users/delete).
package user_delete

import (
	"context"
	"log/slog"

	"github.com/postlog/subgen/internal/oas"
)

// MsgDeleted is the success message. Exported so apitest can assert against it without
// duplicating the text.
const MsgDeleted = "Пользователь удалён"

// Handler deletes a user and deprovisions its panel clients.
type Handler struct {
	svc deleter
}

// New builds the handler.
func New(svc deleter) *Handler { return &Handler{svc: svc} }

// UserDelete implements oas.Handler. DeleteUser surfaces no domain (4xx) error — a missing
// user or a panel/store failure is an internal condition — so any failure is a logged 500.
func (h *Handler) UserDelete(ctx context.Context, req *oas.UserDeleteReq) (oas.UserDeleteRes, error) {
	if err := h.svc.DeleteUser(ctx, req.ID); err != nil {
		slog.Error("handler user_delete: delete failed", "id", req.ID, "err", err)
		return nil, err
	}

	return &oas.MessageResponse{Message: MsgDeleted}, nil
}
