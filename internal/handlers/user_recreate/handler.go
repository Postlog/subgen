// Package user_recreate handles POST /admin/users/recreate.
package user_recreate

import (
	"log/slog"
	"net/http"

	"github.com/postlog/subgen/internal/handlers/web"
)

const msgRecreated = "Клиенты пересозданы"

// Handler re-provisions a user's panel clients.
type Handler struct {
	svc recreator
}

// New builds the handler.
func New(svc recreator) *Handler { return &Handler{svc: svc} }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID int64 `json:"id"`
	}

	if err := web.DecodeJSON(r, &req); err != nil {
		slog.Warn("handler user_recreate: decode failed", "err", err)
		web.WriteJSON(w, false, web.MsgBadRequest)

		return
	}

	id := req.ID

	err := h.svc.RecreateUser(r.Context(), id)
	if err != nil {
		slog.Warn("handler user_recreate: recreate failed", "id", id, "err", err)
	}

	web.JSONResult(w, msgRecreated, err)
}
