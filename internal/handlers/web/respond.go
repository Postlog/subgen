// Package web holds the shared HTTP plumbing for the admin/sub handlers: JSON
// request decoding + responses, user-facing message mapping, the admin session/auth
// middleware, and the HTML renderer (embedded templates + static assets). Each
// action lives in its own internal/handlers/<action> package and depends on this.
package web

import (
	"encoding/json"
	"io"
	"net/http"
)

// MsgBadRequest is the user-facing message for a malformed request body. Handlers
// emit it themselves (web only maps/holds the text; it never writes the response).
const MsgBadRequest = "некорректный JSON в запросе"

// WriteJSON emits {"ok":bool, "msg"|"err":string} (always HTTP 200; the ok flag
// carries success/failure — the admin JS reads it).
func WriteJSON(w http.ResponseWriter, ok bool, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	field := "msg"
	if !ok {
		field = "err"
	}

	_ = json.NewEncoder(w).Encode(map[string]any{"ok": ok, field: msg})
}

// JSON encodes v as the response body (HTTP 200, application/json). Used by the
// admin read endpoints that feed the Vue SPA.
func JSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

// JSONResult writes a success message, or maps an error to user-facing text.
func JSONResult(w http.ResponseWriter, okMsg string, err error) {
	if err != nil {
		WriteJSON(w, false, UserMessage(err))
		return
	}

	WriteJSON(w, true, okMsg)
}

// DecodeJSON decodes the JSON request body into dst and returns any error (the admin
// SPA POSTs JSON, never forms/multipart). It only decodes — the handler logs the
// error and writes the response. The body is capped at 8 MiB (admin payloads are
// tiny). The HTTP method is enforced by the router, so there is no method check here.
func DecodeJSON(r *http.Request, dst any) error {
	return json.NewDecoder(io.LimitReader(r.Body, 8<<20)).Decode(dst)
}
