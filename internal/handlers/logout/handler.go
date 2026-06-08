// Package logout handles POST /admin/api/logout — clears the admin session cookie and
// returns 204. No redirect: the SPA navigates to the login page itself.
package logout

import (
	"context"

	"github.com/postlog/subgen/internal/handlers/web"
	"github.com/postlog/subgen/internal/oas"
)

// Handler clears the admin session cookie.
type Handler struct {
	sess *web.Session
}

// New builds the handler.
func New(sess *web.Session) *Handler { return &Handler{sess: sess} }

// Logout implements oas.Handler: it expires the session cookie via Set-Cookie and
// responds 204 (no body).
func (h *Handler) Logout(_ context.Context) (*oas.LogoutNoContent, error) {
	return &oas.LogoutNoContent{SetCookie: oas.NewOptString(h.sess.ClearCookie().String())}, nil
}
