// Package config_get implements the configGet operation (GET /admin/api/config/mihomo)
// — the mihomo routing config (proxy-groups, rules, rule-providers, base YAML). Without
// ?user it returns the base config; with ?user=<id> that user's custom config (404 if
// none).
package config_get

import (
	"context"
	"log/slog"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/mihomo"
	"github.com/postlog/subgen/internal/oas"
)

// Handler serves a mihomo config (base or a user's custom).
type Handler struct {
	configs configResolver
	routing mihomoReader
}

// New builds the handler.
func New(configs configResolver, routing mihomoReader) *Handler {
	return &Handler{configs: configs, routing: routing}
}

// ConfigGet implements oas.Handler.
func (h *Handler) ConfigGet(ctx context.Context, params oas.ConfigGetParams) (oas.ConfigGetRes, error) {
	configID, found, err := h.resolveConfigID(ctx, params)
	if err != nil {
		slog.Error("handler config_get: resolve config failed", "user", params.User.Or(0), "err", err)
		return nil, err
	}

	if !found {
		// A user scope with no custom config 404s; a base never saved serves empty —
		// empty config means empty (no default substitution).
		if params.User.IsSet() {
			return &oas.ConfigGetNotFound{}, nil
		}

		return &oas.MihomoConfig{Groups: []oas.MihomoGroup{}, Rules: []oas.MihomoRule{}, Providers: []oas.MihomoProvider{}}, nil
	}

	rules, err := h.routing.Rules(ctx, configID)
	if err != nil {
		slog.Error("handler config_get: read rules failed", "configID", configID, "err", err)
		return nil, err
	}

	groups, err := h.routing.ProxyGroups(ctx, configID)
	if err != nil {
		slog.Error("handler config_get: read proxy-groups failed", "configID", configID, "err", err)
		return nil, err
	}

	rps, err := h.routing.RuleProviders(ctx, configID)
	if err != nil {
		slog.Error("handler config_get: read rule-providers failed", "configID", configID, "err", err)
		return nil, err
	}

	baseYAML, err := h.routing.Setting(ctx, configID, "base_yaml")
	if err != nil {
		slog.Error("handler config_get: read base yaml failed", "configID", configID, "err", err)
		return nil, err
	}

	// Profile knobs are returned as stored — no default substitution (an unset config
	// reads back unset). Defaults are applied only when actually serving a subscription.
	profile, err := h.routing.Profile(ctx, configID)
	if err != nil {
		slog.Error("handler config_get: read profile failed", "configID", configID, "err", err)
		return nil, err
	}

	idx := map[int64]int{} // group id -> array index
	for i, g := range groups {
		idx[g.ID] = i
	}

	provIdx := map[int64]int{} // rule_provider id -> array index
	for i, rp := range rps {
		provIdx[rp.ID] = i
	}

	out := &oas.MihomoConfig{
		BaseYAML:              baseYAML,
		ProfileTitle:          profile.Title,
		Filename:              profile.Filename,
		ProfileUpdateInterval: profile.UpdateInterval,
	}

	out.Groups = make([]oas.MihomoGroup, 0, len(groups))
	for _, g := range groups {
		members := make([]oas.PolicyRef, 0, len(g.Members))
		for _, m := range g.Members {
			members = append(members, refToView(m, idx))
		}

		mg := oas.MihomoGroup{Name: g.Name, Type: g.Type.String(), URL: g.URL, Members: members}

		if g.Interval != nil {
			mg.Interval = oas.NewOptInt(*g.Interval)
		}

		if g.Tolerance != nil {
			mg.Tolerance = oas.NewOptInt(*g.Tolerance)
		}

		if g.Lazy != nil {
			mg.Lazy = oas.NewOptBool(*g.Lazy)
		}

		out.Groups = append(out.Groups, mg)
	}

	out.Rules = make([]oas.MihomoRule, 0, len(rules))
	for _, r := range rules {
		mr := oas.MihomoRule{Type: r.Type.String(), Target: refToView(r.Target, idx)}

		if r.Value != nil {
			mr.Value = oas.NewOptString(*r.Value)
		}

		if r.NoResolve != nil {
			mr.NoResolve = oas.NewOptBool(*r.NoResolve)
		}

		// RULE-SET: surface the provider as its array index (real id never leaves).
		if r.ProviderID != nil {
			if i, ok := provIdx[*r.ProviderID]; ok {
				mr.ProviderIdx = oas.NewOptInt(i)
			}
		}

		out.Rules = append(out.Rules, mr)
	}

	out.Providers = make([]oas.MihomoProvider, 0, len(rps))
	for _, rp := range rps {
		out.Providers = append(out.Providers, oas.MihomoProvider{
			Name: rp.Name, Behavior: rp.Behavior, Format: rp.Format,
			URL: rp.URL, Interval: rp.Interval, Mirror: rp.Mirror, MirrorInterval: rp.MirrorInterval,
		})
	}

	return out, nil
}

// resolveConfigID maps the request scope to a config id. No ?user → the base config
// (found=false when never saved). ?user=<id> → that user's custom config (found=false
// when the user has none).
func (h *Handler) resolveConfigID(ctx context.Context, params oas.ConfigGetParams) (int64, bool, error) {
	if !params.User.IsSet() {
		return h.configs.BaseConfigID(ctx, entity.ConfigKindMihomo)
	}

	return h.configs.UserConfigID(ctx, params.User.Value, entity.ConfigKindMihomo)
}

// refToView converts a stored PolicyRef to the wire shape: an inbound ref carries the
// inbound id; a group ref's id becomes its array index.
func refToView(ref mihomo.PolicyRef, idx map[int64]int) oas.PolicyRef {
	v := oas.PolicyRef{Kind: string(ref.Kind)}

	if ref.Kind == mihomo.PolicyInbound && ref.InboundID != nil {
		v.InboundId = oas.NewOptInt64(*ref.InboundID)
	}

	if ref.Kind == mihomo.PolicyGroup && ref.GroupID != nil {
		if i, ok := idx[*ref.GroupID]; ok {
			v.GroupIdx = oas.NewOptInt(i)
		}
	}

	return v
}
