// Package custom_delete implements the customDelete operation
// (POST /admin/api/config/mihomo/custom/delete) — remove a user's custom mihomo config;
// their subscription falls back to the base config.
package custom_delete

import (
	"context"
	"errors"
	"log/slog"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/oas"
)

const msgConfigMissing = "У пользователя нет кастомного конфига"

// Handler drops a user's custom config.
type Handler struct {
	configs configDeleter
}

// New builds the handler.
func New(configs configDeleter) *Handler { return &Handler{configs: configs} }

// CustomDelete implements oas.Handler: a user with no custom config is a 400; any
// unexpected (store) failure is a 500.
func (h *Handler) CustomDelete(ctx context.Context, req *oas.CustomDeleteReq) (oas.CustomDeleteRes, error) {
	if err := h.configs.DeleteUserConfig(ctx, req.UserId, entity.ConfigKindMihomo); err != nil {
		if errors.Is(err, entity.ErrUserConfigNotFound) {
			slog.Warn("handler custom_delete: config missing", "userId", req.UserId)
			return &oas.CustomDeleteBadRequest{ErrMessage: msgConfigMissing}, nil
		}

		slog.Error("handler custom_delete: delete failed", "userId", req.UserId, "err", err)

		return nil, err
	}

	return &oas.MessageResponse{Message: "Кастомный конфиг удалён"}, nil
}
