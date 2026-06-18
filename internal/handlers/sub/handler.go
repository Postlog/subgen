// Package sub handles GET /sub/{kind}/{token} — the per-client subscription. The
// engine (kind) is a path segment so one token serves whatever format the client app
// needs; rendering is delegated to a per-kind engineRenderer (mihomo today).
package sub

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/oas"
	"github.com/postlog/subgen/internal/token"
)

// RenderMeta is the engine-specific response metadata for a rendered subscription.
// The profile fields are per-config (read from the store by the engine renderer, with
// defaults already applied), not service-wide config.
type RenderMeta struct {
	ContentType    string // e.g. "text/yaml; charset=utf-8"
	Filename       string // Content-Disposition filename
	ProfileTitle   string // Profile-Title header (plain text; base64-wrapped here)
	UpdateInterval int    // Profile-Update-Interval, hours
}

// Handler resolves a subscription token to a user, picks their config (custom or
// base) for the requested engine, and delegates rendering to that engine's renderer.
type Handler struct {
	users     usersRepo
	fleet     fleetService
	configs   configsRepo
	renderers map[entity.ConfigKind]EngineRenderer

	secret string
}

// New builds the handler. renderers maps each supported engine kind to its renderer.
func New(users usersRepo, fleet fleetService, configs configsRepo, renderers map[entity.ConfigKind]EngineRenderer, secret string) *Handler {
	return &Handler{
		users: users, fleet: fleet, configs: configs, renderers: renderers,
		secret: secret,
	}
}

// Sub implements oas.Handler. An unknown engine kind or an unmatched token is a 404
// (SubNotFound); a matched token renders the subscriber's profile with the engine's
// headers. Store/render failures bubble up as 5xx via the central ErrorHandler.
//
// Note: ogen pins the 200 media type from the spec (application/yaml), so RenderMeta's
// content type is not echoed per-request — every engine here serves YAML.
func (h *Handler) Sub(ctx context.Context, params oas.SubParams) (oas.SubRes, error) {
	kind := entity.ConfigKind(params.Kind)

	renderer, ok := h.renderers[kind]
	if !ok || params.Token == "" {
		return &oas.SubNotFound{Data: strings.NewReader("not found\n")}, nil
	}

	// Resolve the token against service-owned users only — clients created
	// directly on a panel are not served.
	subIDs, err := h.users.SubIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("users.SubIDs: %w", err)
	}

	var subID string

	for _, id := range subIDs {
		if token.Match(h.secret, id, params.Token) {
			subID = id
			break
		}
	}

	if subID == "" {
		return &oas.SubNotFound{Data: strings.NewReader("not found\n")}, nil
	}

	userID, err := h.users.IDBySubID(ctx, subID)
	if err != nil {
		return nil, fmt.Errorf("users.IDBySubID: %w", err)
	}

	configID, err := h.configID(ctx, userID, kind)
	if err != nil {
		return nil, err
	}

	fleet, err := h.fleet.Fleet(ctx)
	if err != nil {
		return nil, fmt.Errorf("fleet.Fleet: %w", err)
	}

	sub := fleet.Sub(subID)
	if sub == nil {
		sub = &entity.Subscriber{SubID: subID} // provisioned clients missing; serve an empty profile
	}

	body, meta, err := renderer.Render(ctx, sub, configID)
	if err != nil {
		return nil, fmt.Errorf("renderer.Render: %w", err)
	}

	return &oas.SubOKHeaders{
		ProfileUpdateInterval: oas.NewOptString(fmt.Sprintf("%d", meta.UpdateInterval)),
		ProfileTitle:          oas.NewOptString("base64:" + base64.StdEncoding.EncodeToString([]byte(meta.ProfileTitle))),
		ContentDisposition:    oas.NewOptString(fmt.Sprintf("attachment; filename=%q", meta.Filename)),
		SubscriptionUserinfo:  oas.NewOptString(userinfo(sub.Up, sub.Down, sub.Total, sub.Expiry)),
		Response:              oas.SubOK{Data: bytes.NewReader(body)},
	}, nil
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
