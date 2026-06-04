// Package user_delete handles POST /admin/users/delete.
package user_delete

import (
	"log/slog"
	"net/http"

	"github.com/postlog/subgen/internal/handlers/web"
)

const msgDeleted = "Пользователь удалён"

// Handler deletes a user.
type Handler struct {
	svc deleter
}

// New builds the handler.
func New(svc deleter) *Handler { return &Handler{svc: svc} }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID int64 `json:"id"`
	}

	if err := web.DecodeJSON(r, &req); err != nil {
		slog.Warn("handler user_delete: decode failed", "err", err)
		web.WriteJSON(w, false, web.MsgBadRequest)

		return
	}

	id := req.ID

	err := h.svc.DeleteUser(r.Context(), id)
	if err != nil {
		slog.Warn("handler user_delete: delete failed", "id", id, "err", err)
	}

	web.JSONResult(w, msgDeleted, err)
}
