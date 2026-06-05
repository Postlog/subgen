// Package custom_create handles POST /admin/api/config/mihomo/custom/create — create
// a user's custom mihomo config as a snapshot of the base, then edit it separately.
package custom_create

import (
	"log/slog"
	"net/http"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/handlers/web"
)

const msgCreated = "Кастомный конфиг создан"

// Handler creates a per-user custom mihomo config (cloned from the base).
type Handler struct {
	configs configCreator
}

// New builds the handler.
func New(configs configCreator) *Handler { return &Handler{configs: configs} }

type form struct {
	UserID int64 `json:"userId"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var f form
	if err := web.DecodeJSON(r, &f); err != nil || f.UserID == 0 {
		slog.Warn("handler custom_create: decode failed", "err", err)
		web.WriteJSON(w, false, web.MsgBadRequest)

		return
	}

	_, err := h.configs.CreateUserConfig(r.Context(), f.UserID, entity.ConfigKindMihomo)
	if err != nil {
		slog.Warn("handler custom_create: create failed", "userId", f.UserID, "err", err)
	}

	web.JSONResult(w, msgCreated, err)
}
