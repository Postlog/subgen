// Package login handles the admin sign-in. The login PAGE is static HTML served on
// GET /admin/login (LoginPage); the sign-in ACTION is POST /admin/api/login (Login),
// which validates the credentials and, on success, sets the httpOnly session cookie.
// The SPA navigates itself — no server-side redirect on the JSON path.
package login

import (
	"bytes"
	"context"
	"crypto/subtle"
	"log/slog"

	"github.com/postlog/subgen/internal/handlers/web"
	"github.com/postlog/subgen/internal/oas"
)

// MsgBadCredentials is the generic sign-in rejection text (no user/pass distinction).
// Exported so apitest can assert against it without duplicating the text.
//
//nolint:gosec // G101 false positive: a user-facing message, not a hardcoded credential.
const MsgBadCredentials = "Invalid username or password"

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

// LoginPage implements oas.Handler for GET /admin/login: an already-signed-in visitor
// (a valid session cookie) is redirected to the app, otherwise the static login form is
// served.
func (h *Handler) LoginPage(_ context.Context, params oas.LoginPageParams) (oas.LoginPageRes, error) {
	if h.sess.Valid(params.SubgenAdmin.Or("")) {
		return &oas.LoginPageFound{Location: oas.NewOptString("/admin/users")}, nil
	}

	page, err := web.ReadPage(h.staticDir, "login.html")
	if err != nil {
		slog.Error("handler login: read login page failed", "err", err)
		return nil, err
	}

	return &oas.LoginPageOK{Data: bytes.NewReader(page)}, nil
}

// Login implements oas.Handler: valid credentials get a 200 with the session cookie;
// anything else is a 401 with a generic message (no user/pass distinction).
func (h *Handler) Login(_ context.Context, req *oas.LoginReq) (oas.LoginRes, error) {
	userOK := subtle.ConstantTimeCompare([]byte(req.User), []byte(h.user)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(req.Password), []byte(h.pass)) == 1

	if !userOK || !passOK {
		return &oas.ErrorResponse{ErrMessage: MsgBadCredentials}, nil
	}

	return &oas.MessageResponseHeaders{
		SetCookie: oas.NewOptString(h.sess.IssueCookie().String()),
		Response:  oas.MessageResponse{Message: "ok"},
	}, nil
}
