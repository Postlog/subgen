// Package logout handles GET /admin/logout — clears the admin session.
package logout

import (
	"net/http"

	"github.com/postlog/subgen/internal/handlers/web"
)

// Handler clears the admin session and redirects to the login page.
type Handler struct {
	sess *web.Session
}

// New builds the handler.
func New(sess *web.Session) *Handler { return &Handler{sess: sess} }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.sess.Clear(w)
	http.Redirect(w, r, "/admin/login", http.StatusFound)
}
