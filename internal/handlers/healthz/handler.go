// Package healthz implements the healthz operation (GET /healthz) — a liveness probe.
package healthz

import (
	"context"
	"strings"

	"github.com/postlog/subgen/internal/oas"
)

// Handler answers liveness probes.
type Handler struct{}

// New builds the handler.
func New() *Handler { return &Handler{} }

// Healthz implements oas.Handler: a trivial text/plain "ok".
func (h *Handler) Healthz(_ context.Context) (oas.HealthzOK, error) {
	return oas.HealthzOK{Data: strings.NewReader("ok\n")}, nil
}
