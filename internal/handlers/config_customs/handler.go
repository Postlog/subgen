// Package config_customs handles GET /admin/api/config/mihomo/customs — the users
// that have a custom mihomo config, as {userId,name} pairs for the admin scope
// selector.
package config_customs

import (
	"log/slog"
	"net/http"
	"sort"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/handlers/web"
)

// Handler serves the list of users with a custom mihomo config.
type Handler struct {
	configs configLister
	users   userLister
}

// New builds the handler.
func New(configs configLister, users userLister) *Handler {
	return &Handler{configs: configs, users: users}
}

type customView struct {
	UserID int64  `json:"userId"`
	Name   string `json:"name"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ids, err := h.configs.UserConfigUserIDs(ctx, entity.ConfigKindMihomo)
	if err != nil {
		slog.Error("handler config_customs: list config owners failed", "err", err)
		http.Error(w, "store unavailable", http.StatusInternalServerError)

		return
	}

	users, err := h.users.List(ctx)
	if err != nil {
		slog.Error("handler config_customs: list users failed", "err", err)
		http.Error(w, "store unavailable", http.StatusInternalServerError)

		return
	}

	name := make(map[int64]string, len(users))
	for i := range users {
		name[users[i].ID] = users[i].Name
	}

	out := make([]customView, 0, len(ids))
	for _, id := range ids {
		out = append(out, customView{UserID: id, Name: name[id]})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })

	web.JSON(w, map[string]any{"customs": out})
}
