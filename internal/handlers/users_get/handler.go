// Package users_get handles GET /admin/api/users — one filtered, paged slice of the
// users table as JSON for the admin SPA. Query params: q (name substring), inbound
// (repeatable node_inbounds.id, OR-filter), page (1-based), perPage.
package users_get

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/handlers/web"
	"github.com/postlog/subgen/internal/token"
)

const (
	defaultPerPage = 50
	maxPerPage     = 200
)

// Handler serves a page of the users list as JSON.
type Handler struct {
	users  userLister
	fleet  fleetReader
	secret string // HMAC secret for subscription tokens
	base   string // public base URL
}

// New builds the handler.
func New(users userLister, fleet fleetReader, secret, base string) *Handler {
	return &Handler{users: users, fleet: fleet, secret: secret, base: base}
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
	page, perPage := paging(r)
	params := entity.UserListParams{
		NameQuery:  r.URL.Query().Get("q"),
		InboundIDs: inboundIDs(r),
		Limit:      perPage,
		Offset:     (page - 1) * perPage,
	}

	res, err := h.users.ListPage(r.Context(), params)
	if err != nil {
		slog.Error("handler users_get: list failed", "err", err)
		http.Error(w, "store unavailable", http.StatusInternalServerError)

		return
	}

	// Health and traffic both come from the cached fleet (one panel snapshot per node),
	// so the page needs no per-user panel calls. A nil fleet (total outage, no prior
	// good snapshot) yields no badges and zero stats — fleet's methods are nil-safe.
	fleet, _ := h.fleet.Fleet(r.Context())
	base := strings.TrimRight(h.base, "/")

	rows := make([]row, 0, len(res.Users))

	for i := range res.Users {
		u := &res.Users[i]

		inbounds := make([]inboundView, 0, len(u.Connections))
		for _, c := range u.Connections {
			inbounds = append(inbounds, inboundView{
				ID: c.InboundID, Label: c.Node + "-" + c.Name, Port: c.Port,
				Missing: fleet.ClientMissing(c.InboundID, u.Name),
			})
		}

		vr := row{
			ID:       u.ID,
			Name:     u.Name,
			Sub:      subInfo{ID: u.SubID, URL: base + "/sub/mihomo/" + token.Make(h.secret, u.SubID)},
			Inbounds: inbounds,
		}

		if sub := fleet.Sub(u.SubID); sub != nil {
			vr.Stats = stats{Up: sub.Up, Down: sub.Down}
		}

		rows = append(rows, vr)
	}

	web.JSON(w, map[string]any{"users": rows, "total": res.Total, "page": page, "perPage": perPage})
}

// paging reads the 1-based page and clamped perPage from the query (defaults 1 and
// defaultPerPage; perPage clamped to [1, maxPerPage]).
func paging(r *http.Request) (page, perPage int) {
	page = 1
	if v, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && v > 1 {
		page = v
	}

	perPage = defaultPerPage
	if v, err := strconv.Atoi(r.URL.Query().Get("perPage")); err == nil && v > 0 {
		perPage = v
	}

	if perPage > maxPerPage {
		perPage = maxPerPage
	}

	return page, perPage
}

// inboundIDs parses the repeatable ?inbound= filter into node_inbounds ids, dropping
// anything non-numeric.
func inboundIDs(r *http.Request) []int64 {
	var out []int64

	for _, s := range r.URL.Query()["inbound"] {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			out = append(out, id)
		}
	}

	return out
}
