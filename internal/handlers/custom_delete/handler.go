// Package custom_delete implements the customDelete operation
// (POST /admin/api/config/mihomo/custom/delete) — remove a user's custom mihomo config;
// their subscription falls back to the base config.
package custom_delete

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/handlers/web"
	"github.com/postlog/subgen/internal/oas"
)

// Handler drops a user's custom config.
type Handler struct {
	configs configDeleter
}

// New builds the handler.
func New(configs configDeleter) *Handler { return &Handler{configs: configs} }

// CustomDelete implements oas.Handler.
func (h *Handler) CustomDelete(ctx context.Context, req *oas.CustomDeleteReq) (oas.CustomDeleteRes, error) {
	if err := h.configs.DeleteUserConfig(ctx, req.UserId, entity.ConfigKindMihomo); err != nil {
		return &oas.CustomDeleteBadRequest{ErrMessage: web.UserMessage(err)}, nil
	}

	return &oas.MessageResponse{Message: "Кастомный конфиг удалён"}, nil
}
