// Package custom_delete handles POST /admin/api/config/mihomo/custom/delete — remove
// a user's custom mihomo config; their subscription falls back to the base config.
package custom_delete

import (
	"log/slog"
	"net/http"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/handlers/web"
)

const msgDeleted = "Кастомный конфиг удалён"

// Handler deletes a per-user custom mihomo config.
type Handler struct {
	configs configDeleter
}

// New builds the handler.
func New(configs configDeleter) *Handler { return &Handler{configs: configs} }

type form struct {
	UserID int64 `json:"userId"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var f form
	if err := web.DecodeJSON(r, &f); err != nil || f.UserID == 0 {
		slog.Warn("handler custom_delete: decode failed", "err", err)
		web.WriteJSON(w, false, web.MsgBadRequest)

		return
	}

	err := h.configs.DeleteUserConfig(r.Context(), f.UserID, entity.ConfigKindMihomo)
	if err != nil {
		slog.Warn("handler custom_delete: delete failed", "userId", f.UserID, "err", err)
	}

	web.JSONResult(w, msgDeleted, err)
}
