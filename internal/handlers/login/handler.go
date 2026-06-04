// Package login handles GET/POST /admin/login — the admin sign-in. GET serves the
// static login page; POST validates the credentials and returns JSON (the page's
// JS redirects on success).
package login

import (
	"crypto/subtle"
	"log/slog"
	"net/http"

	"github.com/postlog/subgen/internal/handlers/web"
)

// Handler renders and processes the admin login.
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

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			User     string `json:"user"`
			Password string `json:"password"`
		}

		if err := web.DecodeJSON(r, &req); err != nil {
			slog.Warn("handler login: decode failed", "err", err)
			web.WriteJSON(w, false, web.MsgBadRequest)

			return
		}

		userOK := subtle.ConstantTimeCompare([]byte(req.User), []byte(h.user)) == 1
		passOK := subtle.ConstantTimeCompare([]byte(req.Password), []byte(h.pass)) == 1

		if userOK && passOK {
			h.sess.Issue(w)
			web.WriteJSON(w, true, "ok")

			return
		}

		web.WriteJSON(w, false, "Неверный логин или пароль")

		return
	}

	if h.sess.IsAuthed(r) {
		http.Redirect(w, r, "/admin/users", http.StatusFound)
		return
	}

	web.ServePage(w, h.staticDir, "login.html")
}
