// Package sub handles GET /sub/{kind}/{token} — the per-client subscription. The
// engine (kind) is a path segment so one token serves whatever format the client app
// needs; rendering is delegated to a per-kind engineRenderer (mihomo today).
package sub

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/token"
)

// RenderMeta is the engine-specific response metadata for a rendered subscription.
type RenderMeta struct {
	ContentType string // e.g. "text/yaml; charset=utf-8"
	Filename    string // Content-Disposition filename
}

// Handler resolves a subscription token to a user, picks their config (custom or
// base) for the requested engine, and delegates rendering to that engine's renderer.
type Handler struct {
	users     userResolver
	fleet     fleetReader
	configs   configResolver
	renderers map[entity.ConfigKind]EngineRenderer

	secret         string
	profileTitle   string
	updateInterval int
}

// New builds the handler. renderers maps each supported engine kind to its renderer.
func New(users userResolver, fleet fleetReader, configs configResolver, renderers map[entity.ConfigKind]EngineRenderer, secret, profileTitle string, updateInterval int) *Handler {
	return &Handler{
		users: users, fleet: fleet, configs: configs, renderers: renderers,
		secret: secret, profileTitle: profileTitle, updateInterval: updateInterval,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tok := vars["token"]

	kind := entity.ConfigKind(vars["kind"])

	renderer, ok := h.renderers[kind]
	if !ok || tok == "" {
		http.NotFound(w, r)
		return
	}

	ctx := r.Context()

	// Resolve the token against service-owned users only — clients created
	// directly on a panel are not served.
	subIDs, err := h.users.SubIDs(ctx)
	if err != nil {
		slog.Error("handler sub: list sub ids failed", "err", err)
		http.Error(w, "store unavailable", http.StatusInternalServerError)

		return
	}

	var subID string

	for _, id := range subIDs {
		if token.Match(h.secret, id, tok) {
			subID = id
			break
		}
	}

	if subID == "" {
		http.NotFound(w, r)
		return
	}

	userID, err := h.users.IDBySubID(ctx, subID)
	if err != nil {
		slog.Error("handler sub: resolve user failed", "err", err)
		http.Error(w, "store unavailable", http.StatusInternalServerError)

		return
	}

	configID, err := h.configID(ctx, userID, kind)
	if err != nil {
		slog.Error("handler sub: resolve config failed", "err", err)
		http.Error(w, "config unavailable", http.StatusInternalServerError)

		return
	}

	fleet, err := h.fleet.Fleet(ctx)
	if err != nil {
		slog.Error("handler sub: fleet fetch failed", "err", err)
		http.Error(w, "upstream unavailable", http.StatusBadGateway)

		return
	}

	sub := fleet.Sub(subID)
	if sub == nil {
		sub = &entity.Subscriber{SubID: subID} // provisioned clients missing; serve an empty profile
	}

	body, meta, err := renderer.Render(ctx, sub, configID)
	if err != nil {
		slog.Error("handler sub: render failed", "err", err)
		http.Error(w, "render error", http.StatusInternalServerError)

		return
	}

	title := h.profileTitle
	if title == "" {
		title = "Freedom"
	}

	hdr := w.Header()
	hdr.Set("Content-Type", meta.ContentType)
	hdr.Set("Profile-Update-Interval", fmt.Sprintf("%d", h.updateInterval))
	hdr.Set("Profile-Title", "base64:"+base64.StdEncoding.EncodeToString([]byte(title)))
	hdr.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", meta.Filename))
	hdr.Set("Subscription-Userinfo", userinfo(sub.Up, sub.Down, sub.Total, sub.Expiry))

	_, _ = w.Write(body) //nolint:gosec // server-generated subscription bytes (engine YAML), not user HTML; served with an explicit Content-Type
}

// configID picks the config to render: a user's custom config for this engine when
// present, else the engine's base. Returns 0 when neither exists — the renderer then
// reads no content and emits a minimal profile (just the subscriber's proxies).
func (h *Handler) configID(ctx context.Context, userID int64, kind entity.ConfigKind) (int64, error) {
	if id, ok, err := h.configs.UserConfigID(ctx, userID, kind); err != nil {
		return 0, err
	} else if ok {
		return id, nil
	}

	if id, ok, err := h.configs.BaseConfigID(ctx, kind); err != nil {
		return 0, err
	} else if ok {
		return id, nil
	}

	return 0, nil
}

// userinfo formats the Subscription-Userinfo header. Expiry is ms epoch; we emit
// seconds. expire is omitted when 0 (no expiry) so clients don't render "1970".
func userinfo(up, down, total, expiryMs int64) string {
	s := fmt.Sprintf("upload=%d; download=%d; total=%d", up, down, total)
	if expiryMs > 0 {
		s += fmt.Sprintf("; expire=%d", expiryMs/1000)
	}

	return s
}
