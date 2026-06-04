// Package healthz handles GET /healthz — a trivial liveness probe.
package healthz

import "net/http"

// Handler answers liveness probes.
type Handler struct{}

// New builds the handler.
func New() *Handler { return &Handler{} }

func (h *Handler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte("ok\n"))
}
