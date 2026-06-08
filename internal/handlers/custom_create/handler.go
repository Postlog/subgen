// Package custom_create implements the customCreate operation
// (POST /admin/api/config/mihomo/custom/create) — create a user's custom mihomo config
// as a snapshot of the base, then edit it separately.
package custom_create

import (
	"context"
	"errors"
	"log/slog"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/oas"
)

const msgConfigExists = "У пользователя уже есть кастомный конфиг"

// Handler clones the base config into a new per-user custom config.
type Handler struct {
	configs configCreator
}

// New builds the handler.
func New(configs configCreator) *Handler { return &Handler{configs: configs} }

// CustomCreate implements oas.Handler: an already-existing custom config is a 400; any
// unexpected (store) failure is a 500.
func (h *Handler) CustomCreate(ctx context.Context, req *oas.CustomCreateReq) (oas.CustomCreateRes, error) {
	if _, err := h.configs.CreateUserConfig(ctx, req.UserId, entity.ConfigKindMihomo); err != nil {
		if errors.Is(err, entity.ErrUserConfigExists) {
			slog.Warn("handler custom_create: config already exists", "userId", req.UserId)
			return &oas.CustomCreateBadRequest{ErrMessage: msgConfigExists}, nil
		}

		slog.Error("handler custom_create: create failed", "userId", req.UserId, "err", err)

		return nil, err
	}

	return &oas.MessageResponse{Message: "Кастомный конфиг создан"}, nil
}
