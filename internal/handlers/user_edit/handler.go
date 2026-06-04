// Package user_edit handles POST /admin/users/edit.
package user_edit

import (
	"log/slog"
	"net/http"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/handlers/web"
)

const msgUpdated = "Подключения обновлены"

// Handler updates a user's connections from the edit form.
type Handler struct {
	svc editor
}

// New builds the handler.
func New(svc editor) *Handler { return &Handler{svc: svc} }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID         int64   `json:"id"`
		InboundIDs []int64 `json:"inboundIDs"`
	}

	if err := web.DecodeJSON(r, &req); err != nil {
		slog.Warn("handler user_edit: decode failed", "err", err)
		web.WriteJSON(w, false, web.MsgBadRequest)

		return
	}

	id := req.ID
	sel := entity.ConnectionSelection{InboundIDs: req.InboundIDs}

	err := h.svc.EditUser(r.Context(), id, sel)
	if err != nil {
		slog.Warn("handler user_edit: edit failed", "id", id, "err", err)
	}

	web.JSONResult(w, msgUpdated, err)
}
