// Package rules handles GET /rules/{file} — mirrored rule-provider files.
package rules

import (
	"net/http"

	"github.com/gorilla/mux"
)

// Handler serves mirrored rule-provider files from memory.
type Handler struct {
	mirror ruleFiles
}

// New builds the handler.
func New(mirror ruleFiles) *Handler { return &Handler{mirror: mirror} }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	file := mux.Vars(r)["file"]
	if file == "" {
		http.NotFound(w, r)
		return
	}

	data, ctype, ok := h.mirror.Get(file)
	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", ctype)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data) //nolint:gosec // operator-configured rule-provider bytes, not user HTML; served with explicit type + nosniff
}
