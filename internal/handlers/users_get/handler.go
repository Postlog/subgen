// Package users_get implements the usersGet operation (GET /admin/api/users) — one
// filtered, paged slice of the users table for the admin SPA.
package users_get

import (
	"context"
	"log/slog"
	"strings"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/oas"
	"github.com/postlog/subgen/internal/token"
)

const (
	defaultPerPage = 50
	maxPerPage     = 200
)

// Handler serves a page of the users list.
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

// UsersGet implements oas.Handler. Health badges and traffic come from the cached
// fleet (one panel snapshot per node), so the page needs no per-user panel calls.
func (h *Handler) UsersGet(ctx context.Context, params oas.UsersGetParams) (oas.UsersGetRes, error) {
	page := params.Page.Or(1)
	if page < 1 {
		page = 1
	}

	perPage := params.PerPage.Or(defaultPerPage)
	if perPage < 1 {
		perPage = defaultPerPage
	}

	if perPage > maxPerPage {
		perPage = maxPerPage
	}

	res, err := h.users.ListPage(ctx, entity.UserListParams{
		NameQuery: params.Q.Or(""), InboundIDs: params.Inbound,
		Limit: perPage, Offset: (page - 1) * perPage,
	})
	if err != nil {
		slog.Error("handler users_get: list page failed", "page", page, "perPage", perPage, "err", err)
		return nil, err
	}

	fl, _ := h.fleet.Fleet(ctx)
	base := strings.TrimRight(h.base, "/")

	rows := make([]oas.UsersGetOKUsersItem, 0, len(res.Users))

	for i := range res.Users {
		u := &res.Users[i]

		inbounds := make([]oas.UsersGetOKUsersItemInboundsItem, 0, len(u.Connections))
		for _, c := range u.Connections {
			inbounds = append(inbounds, oas.UsersGetOKUsersItemInboundsItem{
				ID: c.InboundID, Label: c.Node + "-" + c.Name, Port: c.Port,
				Missing: fl.ClientMissing(c.InboundID, u.Name),
			})
		}

		row := oas.UsersGetOKUsersItem{
			ID: u.ID, Name: u.Name, Inbounds: inbounds,
			Sub: oas.UsersGetOKUsersItemSub{ID: u.SubID, URL: base + "/sub/mihomo/" + token.Make(h.secret, u.SubID)},
		}

		if u.Description != nil {
			row.Description = oas.NewOptString(*u.Description)
		}

		if sub := fl.Sub(u.SubID); sub != nil {
			row.Stats = oas.UsersGetOKUsersItemStats{Up: sub.Up, Down: sub.Down}
		}

		rows = append(rows, row)
	}

	return &oas.UsersGetOK{Users: rows, Total: res.Total, Page: page, PerPage: perPage}, nil
}
