// Package rules handles GET /rules/{file} — mirrored rule-provider files.
package rules

import (
	"bytes"
	"context"
	"strings"

	"github.com/postlog/subgen/internal/oas"
)

// Handler serves mirrored rule-provider files from memory.
type Handler struct {
	mirror rulesetMirror
}

// New builds the handler.
func New(mirror rulesetMirror) *Handler { return &Handler{mirror: mirror} }

// Rules implements oas.Handler. A missing/unknown file is a 404 (RulesNotFound); a hit
// streams the mirrored bytes with nosniff.
//
// Note: ogen pins the 200 media type from the spec (application/octet-stream), so the
// mirror's per-file content type is not echoed per-request.
func (h *Handler) Rules(_ context.Context, params oas.RulesParams) (oas.RulesRes, error) {
	if params.File == "" {
		return &oas.RulesNotFound{Data: strings.NewReader("not found\n")}, nil
	}

	data, _, ok := h.mirror.Get(params.File)
	if !ok {
		return &oas.RulesNotFound{Data: strings.NewReader("not found\n")}, nil
	}

	return &oas.RulesOKHeaders{
		XContentTypeOptions: oas.NewOptString("nosniff"),
		Response:            oas.RulesOK{Data: bytes.NewReader(data)},
	}, nil
}
