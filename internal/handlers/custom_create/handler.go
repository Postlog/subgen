// Package custom_create implements the customCreate operation
// (POST /admin/api/config/mihomo/custom/create) — create a user's custom mihomo config
// as a snapshot of the base, then edit it separately.
package custom_create

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/handlers/web"
	"github.com/postlog/subgen/internal/oas"
)

// Handler clones the base config into a new per-user custom config.
type Handler struct {
	configs configCreator
}

// New builds the handler.
func New(configs configCreator) *Handler { return &Handler{configs: configs} }

// CustomCreate implements oas.Handler.
func (h *Handler) CustomCreate(ctx context.Context, req *oas.CustomCreateReq) (oas.CustomCreateRes, error) {
	if _, err := h.configs.CreateUserConfig(ctx, req.UserId, entity.ConfigKindMihomo); err != nil {
		return &oas.CustomCreateBadRequest{ErrMessage: web.UserMessage(err)}, nil
	}

	return &oas.MessageResponse{Message: "Кастомный конфиг создан"}, nil
}
