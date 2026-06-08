// Package api assembles the per-action handlers into the single oas.Handler ogen
// needs: a thin composite that forwards each operation to its own handler (each keeps
// its narrow deps), plus the shared oas.SecurityHandler (admin session cookie) and
// ErrorHandler.
//
// Each operation is explicitly forwarded (the composite does NOT embed
// oas.UnimplementedHandler), so adding an operation to the spec fails to compile until
// a handler is wired here. Per-operation business errors are returned as typed responses
// by the handlers, which log their own failures with context; the central ErrorHandler
// only handles what bypasses the handlers — security and request-decoding failures.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/ogen-go/ogen/ogenerrors"

	"github.com/postlog/subgen/internal/handlers/config_customs"
	"github.com/postlog/subgen/internal/handlers/config_get"
	"github.com/postlog/subgen/internal/handlers/config_save"
	"github.com/postlog/subgen/internal/handlers/config_schema"
	"github.com/postlog/subgen/internal/handlers/custom_create"
	"github.com/postlog/subgen/internal/handlers/custom_delete"
	"github.com/postlog/subgen/internal/handlers/healthz"
	"github.com/postlog/subgen/internal/handlers/login"
	"github.com/postlog/subgen/internal/handlers/logout"
	"github.com/postlog/subgen/internal/handlers/node_delete"
	"github.com/postlog/subgen/internal/handlers/node_save"
	"github.com/postlog/subgen/internal/handlers/nodes_get"
	"github.com/postlog/subgen/internal/handlers/provider_check"
	"github.com/postlog/subgen/internal/handlers/rules"
	"github.com/postlog/subgen/internal/handlers/sub"
	"github.com/postlog/subgen/internal/handlers/user_create"
	"github.com/postlog/subgen/internal/handlers/user_delete"
	"github.com/postlog/subgen/internal/handlers/user_edit"
	"github.com/postlog/subgen/internal/handlers/user_recreate"
	"github.com/postlog/subgen/internal/handlers/users_get"
	"github.com/postlog/subgen/internal/handlers/web"
	"github.com/postlog/subgen/internal/oas"
)

// errUnauthorized is returned by the security handler for an absent/invalid session;
// ErrorHandler maps it (and ogen's own SecurityError) to 401.
var errUnauthorized = errors.New("unauthorized")

// Handlers bundles every per-action handler the composite forwards to. main builds it
// in the composition root; it grows as operations are migrated.
type Handlers struct {
	Healthz *healthz.Handler

	Sub   *sub.Handler
	Rules *rules.Handler

	Login  *login.Handler
	Logout *logout.Handler

	UsersGet     *users_get.Handler
	UserCreate   *user_create.Handler
	UserEdit     *user_edit.Handler
	UserDelete   *user_delete.Handler
	UserRecreate *user_recreate.Handler

	NodesGet   *nodes_get.Handler
	NodeSave   *node_save.Handler
	NodeDelete *node_delete.Handler

	ConfigGet     *config_get.Handler
	ConfigSchema  *config_schema.Handler
	ConfigCustoms *config_customs.Handler
	ConfigSave    *config_save.Handler
	CustomCreate  *custom_create.Handler
	CustomDelete  *custom_delete.Handler
	ProviderCheck *provider_check.Handler
}

// Server implements oas.Handler (by forwarding to the per-action handlers) and
// oas.SecurityHandler.
type Server struct {
	h       Handlers
	session *web.Session
}

// New builds the composite from the session and the per-action handlers.
func New(session *web.Session, h Handlers) *Server { return &Server{h: h, session: session} }

// ---- forwarded operations -------------------------------------------------------

func (s *Server) Healthz(ctx context.Context) (oas.HealthzOK, error) {
	return s.h.Healthz.Healthz(ctx)
}

func (s *Server) Sub(ctx context.Context, params oas.SubParams) (oas.SubRes, error) {
	return s.h.Sub.Sub(ctx, params)
}

func (s *Server) Rules(ctx context.Context, params oas.RulesParams) (oas.RulesRes, error) {
	return s.h.Rules.Rules(ctx, params)
}

func (s *Server) Login(ctx context.Context, req *oas.LoginReq) (oas.LoginRes, error) {
	return s.h.Login.Login(ctx, req)
}

func (s *Server) Logout(ctx context.Context) (*oas.LogoutNoContent, error) {
	return s.h.Logout.Logout(ctx)
}

func (s *Server) UsersGet(ctx context.Context, params oas.UsersGetParams) (oas.UsersGetRes, error) {
	return s.h.UsersGet.UsersGet(ctx, params)
}

func (s *Server) UserCreate(ctx context.Context, req *oas.UserCreateReq) (oas.UserCreateRes, error) {
	return s.h.UserCreate.UserCreate(ctx, req)
}

func (s *Server) UserEdit(ctx context.Context, req *oas.UserEditReq) (oas.UserEditRes, error) {
	return s.h.UserEdit.UserEdit(ctx, req)
}

func (s *Server) UserDelete(ctx context.Context, req *oas.UserDeleteReq) (oas.UserDeleteRes, error) {
	return s.h.UserDelete.UserDelete(ctx, req)
}

func (s *Server) UserRecreate(ctx context.Context, req *oas.UserRecreateReq) (oas.UserRecreateRes, error) {
	return s.h.UserRecreate.UserRecreate(ctx, req)
}

func (s *Server) NodesGet(ctx context.Context) (oas.NodesGetRes, error) {
	return s.h.NodesGet.NodesGet(ctx)
}

func (s *Server) NodeSave(ctx context.Context, req *oas.NodeSaveReq) (oas.NodeSaveRes, error) {
	return s.h.NodeSave.NodeSave(ctx, req)
}

func (s *Server) NodeDelete(ctx context.Context, req *oas.NodeDeleteReq) (oas.NodeDeleteRes, error) {
	return s.h.NodeDelete.NodeDelete(ctx, req)
}

func (s *Server) ConfigGet(ctx context.Context, params oas.ConfigGetParams) (oas.ConfigGetRes, error) {
	return s.h.ConfigGet.ConfigGet(ctx, params)
}

func (s *Server) ConfigSchema(ctx context.Context) (oas.ConfigSchemaRes, error) {
	return s.h.ConfigSchema.ConfigSchema(ctx)
}

func (s *Server) ConfigCustoms(ctx context.Context) (oas.ConfigCustomsRes, error) {
	return s.h.ConfigCustoms.ConfigCustoms(ctx)
}

func (s *Server) ConfigSave(ctx context.Context, req *oas.ConfigSaveReq) (oas.ConfigSaveRes, error) {
	return s.h.ConfigSave.ConfigSave(ctx, req)
}

func (s *Server) CustomCreate(ctx context.Context, req *oas.CustomCreateReq) (oas.CustomCreateRes, error) {
	return s.h.CustomCreate.CustomCreate(ctx, req)
}

func (s *Server) CustomDelete(ctx context.Context, req *oas.CustomDeleteReq) (oas.CustomDeleteRes, error) {
	return s.h.CustomDelete.CustomDelete(ctx, req)
}

func (s *Server) ProviderCheck(ctx context.Context, req *oas.ProviderCheckReq) (oas.ProviderCheckRes, error) {
	return s.h.ProviderCheck.ProviderCheck(ctx, req)
}

// ---- security + errors ----------------------------------------------------------

// HandleAdminSession validates the admin session cookie for the secured operations.
func (s *Server) HandleAdminSession(ctx context.Context, _ oas.OperationName, t oas.AdminSession) (context.Context, error) {
	if s.session.Valid(t.APIKey) {
		return ctx, nil
	}

	return ctx, errUnauthorized
}

// ErrorHandler maps the failures that bypass the operation handlers — an absent/invalid
// session and a malformed request — to an idiomatic 4xx with a generic {errMessage},
// logging each (there is no handler context to log them otherwise). A 500 here is a
// plain error a handler returned; the handler already logged it with its own operation
// context, so it is NOT re-logged generically.
func (s *Server) ErrorHandler(_ context.Context, w http.ResponseWriter, r *http.Request, err error) {
	status, msg := http.StatusInternalServerError, "Внутренняя ошибка"

	var secErr *ogenerrors.SecurityError

	switch {
	case errors.Is(err, errUnauthorized), errors.As(err, &secErr):
		status, msg = http.StatusUnauthorized, "Требуется авторизация"

		slog.Warn("api: unauthorized request", "path", r.URL.Path)
	case isBadRequest(err):
		status, msg = http.StatusBadRequest, "Некорректный запрос"

		slog.Warn("api: malformed request", "path", r.URL.Path, "err", err)
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
