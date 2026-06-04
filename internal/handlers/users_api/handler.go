// Package users_api handles GET /admin/api/users — the users table as JSON for the
// admin SPA.
package users_api

import (
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/postlog/subgen/internal/handlers/web"
	"github.com/postlog/subgen/internal/token"
)

// Handler serves the users list as JSON.
type Handler struct {
	users  userLister
	fleet  fleetReader
	health connHealth
	secret string // HMAC secret for subscription tokens
	base   string // public base URL
}

// New builds the handler.
func New(users userLister, fleet fleetReader, health connHealth, secret, base string) *Handler {
	return &Handler{users: users, fleet: fleet, health: health, secret: secret, base: base}
}

type subInfo struct {
	ID  string `json:"id"`  // subId
	URL string `json:"url"` // <publicBase>/sub/<token>
}

type inboundView struct {
	ID      int64  `json:"id"`    // node_inbounds.id
	Label   string `json:"label"` // "<node name>-<inbound name>"
	Port    int    `json:"port"`
	Missing bool   `json:"missing"`
}

type stats struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

type row struct {
	ID       int64         `json:"id"`
	Name     string        `json:"name"`
	Sub      subInfo       `json:"sub"`
	Inbounds []inboundView `json:"inbounds"`
	Stats    stats         `json:"stats"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	users, err := h.users.List(r.Context())
	if err != nil {
		slog.Error("handler users_api: list failed", "err", err)
		http.Error(w, "store unavailable", http.StatusInternalServerError)

		return
	}

	fleet, _ := h.fleet.Fleet(r.Context())
	base := strings.TrimRight(h.base, "/")

	rows := make([]row, 0, len(users))

	for i := range users {
		u := &users[i]

		missing := map[int64]bool{}
		for _, c := range h.health.MissingConnections(r.Context(), u) {
			missing[c.InboundID] = true
		}

		inbounds := make([]inboundView, 0, len(u.Connections))
		for _, c := range u.Connections {
			inbounds = append(inbounds, inboundView{
				ID: c.InboundID, Label: c.Node + "-" + c.Name, Port: c.Port, Missing: missing[c.InboundID],
			})
		}

		vr := row{
			ID:       u.ID,
			Name:     u.Name,
			Sub:      subInfo{ID: u.SubID, URL: base + "/sub/" + token.Make(h.secret, u.SubID)},
			Inbounds: inbounds,
		}

		if fleet != nil {
			if sub := fleet.Sub(u.SubID); sub != nil {
				vr.Stats = stats{Up: sub.Up, Down: sub.Down}
			}
		}

		rows = append(rows, vr)
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })

	web.JSON(w, map[string]any{"users": rows})
}
