// Package api assembles the per-action handlers into the single oas.Handler ogen
// needs: a thin composite that forwards each operation to its own handler (each keeps
// its narrow deps), plus the shared oas.SecurityHandler (admin session cookie) and
// ErrorHandler (security/decoding failures -> idiomatic 4xx/5xx + {errMessage}).
//
// Operations not yet migrated fall back to oas.UnimplementedHandler (501); main does
// not route those paths to this server, so they are never reached during the migration.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/ogen-go/ogen/ogenerrors"

	"github.com/postlog/subgen/internal/handlers/healthz"
	"github.com/postlog/subgen/internal/handlers/node_delete"
	"github.com/postlog/subgen/internal/handlers/nodes_get"
	"github.com/postlog/subgen/internal/handlers/web"
	"github.com/postlog/subgen/internal/oas"
)

// errUnauthorized is returned by the security handler for an absent/invalid session;
// ErrorHandler maps it (and ogen's own SecurityError) to 401.
var errUnauthorized = errors.New("unauthorized")

// Handlers bundles every per-action handler the composite forwards to. main builds it
// in the composition root; it grows as operations are migrated.
type Handlers struct {
	Healthz    *healthz.Handler
	NodesGet   *nodes_get.Handler
	NodeDelete *node_delete.Handler
}

// Server implements oas.Handler (by forwarding to the per-action handlers) and
// oas.SecurityHandler.
type Server struct {
	oas.UnimplementedHandler
	h       Handlers
	session *web.Session
}

// New builds the composite from the session and the per-action handlers.
func New(session *web.Session, h Handlers) *Server { return &Server{h: h, session: session} }

// ---- forwarded operations -------------------------------------------------------

func (s *Server) Healthz(ctx context.Context) (oas.HealthzOK, error) {
	return s.h.Healthz.Healthz(ctx)
}

func (s *Server) NodesGet(ctx context.Context) (oas.NodesGetRes, error) {
	return s.h.NodesGet.NodesGet(ctx)
}

func (s *Server) NodeDelete(ctx context.Context, req *oas.NodeDeleteReq) (oas.NodeDeleteRes, error) {
	return s.h.NodeDelete.NodeDelete(ctx, req)
}

// ---- security + errors ----------------------------------------------------------

// HandleAdminSession validates the admin session cookie for the secured operations.
func (s *Server) HandleAdminSession(ctx context.Context, _ oas.OperationName, t oas.AdminSession) (context.Context, error) {
	if s.session.Valid(t.APIKey) {
		return ctx, nil
	}

	return ctx, errUnauthorized
}

// ErrorHandler maps security/decoding/validation failures to idiomatic 4xx/5xx with a
// generic {errMessage}. Per-operation business errors are returned as typed responses
// by the handlers, not here. 5xx is logged (the only place these errors surface).
func (s *Server) ErrorHandler(_ context.Context, w http.ResponseWriter, r *http.Request, err error) {
	status, msg := http.StatusInternalServerError, "Внутренняя ошибка"

	var secErr *ogenerrors.SecurityError

	switch {
	case errors.Is(err, errUnauthorized), errors.As(err, &secErr):
		status, msg = http.StatusUnauthorized, "Требуется авторизация"
	case isBadRequest(err):
		status, msg = http.StatusBadRequest, "Некорректный запрос"
	}

	if status >= http.StatusInternalServerError {
		slog.Error("api: request failed", "path", r.URL.Path, "err", err)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(oas.ErrorResponse{ErrMessage: msg})
}

// isBadRequest reports whether err is an ogen request/params decode or validation
// failure (client sent something malformed or schema-invalid).
func isBadRequest(err error) bool {
	var dec *ogenerrors.DecodeRequestError

	var par *ogenerrors.DecodeParamsError

	return errors.As(err, &dec) || errors.As(err, &par)
}

// ensure the interfaces are satisfied.
var (
	_ oas.Handler         = (*Server)(nil)
	_ oas.SecurityHandler = (*Server)(nil)
)
