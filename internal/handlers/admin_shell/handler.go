// Package admin_shell serves the admin SPA shell (index.html) for GET /admin and the
// client-side view routes GET /admin/{view}. A signed-in visitor gets the shell; anyone
// else is redirected to the login page. The view segment is rendered entirely by the SPA
// — the server serves the same index.html for every view.
package admin_shell

import (
	"bytes"
	"context"
	"log/slog"

	"github.com/postlog/subgen/internal/handlers/web"
	"github.com/postlog/subgen/internal/oas"
)

// Handler serves the SPA shell behind the admin session.
type Handler struct {
	sess      *web.Session
	staticDir string
}

// New builds the handler.
func New(sess *web.Session, staticDir string) *Handler {
	return &Handler{sess: sess, staticDir: staticDir}
}

// AdminShell implements oas.Handler for GET /admin.
func (h *Handler) AdminShell(_ context.Context, params oas.AdminShellParams) (oas.AdminShellRes, error) {
	if !h.sess.Valid(params.SubgenAdmin.Or("")) {
		return &oas.AdminShellFound{Location: oas.NewOptString("/admin/login")}, nil
	}

	page, err := h.shell()
	if err != nil {
		return nil, err
	}

	return &oas.AdminShellOK{Data: page}, nil
}

// AdminShellView implements oas.Handler for GET /admin/{view} — the same shell, behind
// the same session check (the {view} segment is a client-side route).
func (h *Handler) AdminShellView(_ context.Context, params oas.AdminShellViewParams) (oas.AdminShellViewRes, error) {
	if !h.sess.Valid(params.SubgenAdmin.Or("")) {
		return &oas.AdminShellViewFound{Location: oas.NewOptString("/admin/login")}, nil
	}

	page, err := h.shell()
	if err != nil {
		return nil, err
	}

	return &oas.AdminShellViewOK{Data: page}, nil
}

// shell reads index.html, logging+propagating a read failure as a 500.
func (h *Handler) shell() (*bytes.Reader, error) {
	page, err := web.ReadPage(h.staticDir, "index.html")
	if err != nil {
		slog.Error("handler admin_shell: read shell failed", "err", err)
		return nil, err
	}

	return bytes.NewReader(page), nil
}
