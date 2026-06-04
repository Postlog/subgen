// Package user_create handles POST /admin/users/create.
package user_create

import (
	"log/slog"
	"net/http"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/handlers/web"
)

const msgCreated = "Создан пользователь"

// Handler creates a user from the new-user form.
type Handler struct {
	svc creator
}

// New builds the handler.
func New(svc creator) *Handler { return &Handler{svc: svc} }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string  `json:"name"`
		InboundIDs []int64 `json:"inboundIDs"`
	}

	if err := web.DecodeJSON(r, &req); err != nil {
		slog.Warn("handler user_create: decode failed", "err", err)
		web.WriteJSON(w, false, web.MsgBadRequest)

		return
	}

	name := req.Name
	sel := entity.ConnectionSelection{InboundIDs: req.InboundIDs}

	_, err := h.svc.CreateUser(r.Context(), name, sel)
	if err != nil {
		slog.Warn("handler user_create: create failed", "name", name, "err", err)
	}

	web.JSONResult(w, msgCreated, err)
}
