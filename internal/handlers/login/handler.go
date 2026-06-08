// Package login handles the admin sign-in. The login PAGE is static HTML served on
// GET /admin/login (ServeHTTP); the sign-in ACTION is POST /admin/api/login (Login),
// which validates the credentials and, on success, sets the httpOnly session cookie.
// The SPA navigates itself — no server-side redirect on the JSON path.
package login

import (
	"context"
	"crypto/subtle"
	"net/http"

	"github.com/postlog/subgen/internal/handlers/web"
	"github.com/postlog/subgen/internal/oas"
)

// Handler renders the login page (GET) and processes the sign-in action (POST).
type Handler struct {
	sess      *web.Session
	user      string
	pass      string
	staticDir string
}

// New builds the handler.
func New(sess *web.Session, user, pass, staticDir string) *Handler {
	return &Handler{sess: sess, user: user, pass: pass, staticDir: staticDir}
}

// ServeHTTP serves the login page on GET /admin/login: an already-authed visitor is
// bounced to the app, otherwise the static login form is served.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.sess.IsAuthed(r) {
		http.Redirect(w, r, "/admin/users", http.StatusFound)
		return
	}

	web.ServePage(w, h.staticDir, "login.html")
}

// Login implements oas.Handler: valid credentials get a 200 with the session cookie;
// anything else is a 401 with a generic message (no user/pass distinction).
func (h *Handler) Login(_ context.Context, req *oas.LoginReq) (oas.LoginRes, error) {
	userOK := subtle.ConstantTimeCompare([]byte(req.User), []byte(h.user)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(req.Password), []byte(h.pass)) == 1

	if !userOK || !passOK {
		return &oas.ErrorResponse{ErrMessage: "Неверный логин или пароль"}, nil
	}

	return &oas.MessageResponseHeaders{
		SetCookie: oas.NewOptString(h.sess.IssueCookie().String()),
		Response:  oas.MessageResponse{Message: "ok"},
	}, nil
}
