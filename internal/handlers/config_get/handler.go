// Package config_get handles GET /admin/api/config/mihomo — the mihomo routing config
// (proxy-groups, routing rules, rule-providers, base YAML) as JSON for the admin SPA.
// Without a query it returns the base config; with ?user=<id> it returns that user's
// custom config (404 if the user has none).
package config_get

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/handlers/web"
	"github.com/postlog/subgen/internal/mihomo"
)

// Handler serves a mihomo config (base or a user's custom) as JSON.
type Handler struct {
	configs configResolver
	routing mihomoReader
}

// New builds the handler.
func New(configs configResolver, routing mihomoReader) *Handler {
	return &Handler{configs: configs, routing: routing}
}

type provider struct {
	Name           string `json:"name"`
	Behavior       string `json:"behavior"`
	Format         string `json:"format"`
	URL            string `json:"url"`
	Interval       int    `json:"interval"`
	Mirror         bool   `json:"mirror"`
	MirrorInterval int    `json:"mirrorInterval"`
}

// refView is a PolicyRef for the wire: built-in kinds carry nothing extra, force
// carries the (real) inbound id, group carries the INDEX of the referenced group in
// the groups array (the frontend works in indices; ids never leave the backend).
type refView struct {
	Kind      string `json:"kind"`
	InboundID *int64 `json:"inboundId,omitempty"`
	GroupIdx  *int   `json:"groupIdx,omitempty"`
}

type ruleView struct {
	Type      string  `json:"type"`
	Value     string  `json:"value"`
	NoResolve bool    `json:"noResolve"`
	Target    refView `json:"target"`
}

type groupView struct {
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	URL       string    `json:"url"`
	Interval  int       `json:"interval"`
	Tolerance int       `json:"tolerance"`
	Lazy      bool      `json:"lazy"`
	Members   []refView `json:"members"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	configID, found, err := h.resolveConfigID(ctx, r)
	if err != nil {
		slog.Error("handler config_get: resolve scope failed", "err", err)
		http.Error(w, "store unavailable", http.StatusInternalServerError)

		return
	}

	if !found {
		// A user scope with no custom config, or a base that was never saved. For the
		// base we serve an empty config (the editor opens blank); for a missing user
		// custom we 404 — the SPA only requests one it listed.
		if r.URL.Query().Get("user") != "" {
			http.NotFound(w, r)
			return
		}

		web.JSON(w, map[string]any{
			"rules": []ruleView{}, "groups": []groupView{}, "providers": []provider{}, "baseYAML": "",
		})

		return
	}

	rules, _ := h.routing.Rules(ctx, configID)
	groups, _ := h.routing.ProxyGroups(ctx, configID)
	rps, _ := h.routing.RuleProviders(ctx, configID)
	baseYAML, _ := h.routing.Setting(ctx, configID, "base_yaml")

	idx := map[int64]int{} // group id -> array index
	for i, g := range groups {
		idx[g.ID] = i
	}

	groupViews := make([]groupView, 0, len(groups))
	for _, g := range groups {
		members := make([]refView, 0, len(g.Members))
		for _, m := range g.Members {
			members = append(members, refToView(m, idx))
		}

		groupViews = append(groupViews, groupView{
			Name: g.Name, Type: g.Type.String(), URL: g.URL,
			Interval: g.Interval, Tolerance: g.Tolerance, Lazy: g.Lazy, Members: members,
		})
	}

	ruleViews := make([]ruleView, 0, len(rules))
	for _, r := range rules {
		ruleViews = append(ruleViews, ruleView{
			Type: r.Type.String(), Value: r.Value, NoResolve: r.NoResolve,
			Target: refToView(r.Target, idx),
		})
	}

	provs := make([]provider, 0, len(rps))
	for _, rp := range rps {
		provs = append(provs, provider{
			Name: rp.Name, Behavior: rp.Behavior, Format: rp.Format,
			URL: rp.URL, Interval: rp.Interval, Mirror: rp.Mirror, MirrorInterval: rp.MirrorInterval,
		})
	}

	web.JSON(w, map[string]any{
		"rules": ruleViews, "groups": groupViews, "providers": provs, "baseYAML": baseYAML,
	})
}

// resolveConfigID maps the request scope to a config id. No ?user → the base config
// (found=false when it was never saved). ?user=<id> → that user's custom config
// (found=false when the user has none). An unparseable ?user is treated as missing.
func (h *Handler) resolveConfigID(ctx context.Context, r *http.Request) (int64, bool, error) {
	q := r.URL.Query().Get("user")
	if q == "" {
		return h.configs.BaseConfigID(ctx, entity.ConfigKindMihomo)
	}

	userID, err := strconv.ParseInt(q, 10, 64)
	if err != nil {
		return 0, false, nil
	}

	return h.configs.UserConfigID(ctx, userID, entity.ConfigKindMihomo)
}

// refToView converts a stored PolicyRef to the wire shape: an inbound ref carries the
// inbound id; a group ref's id becomes its array index.
func refToView(ref mihomo.PolicyRef, idx map[int64]int) refView {
	v := refView{Kind: string(ref.Kind)}

	if ref.Kind == mihomo.PolicyInbound {
		v.InboundID = ref.InboundID
	}

	if ref.Kind == mihomo.PolicyGroup && ref.GroupID != nil {
		if i, ok := idx[*ref.GroupID]; ok {
			v.GroupIdx = &i
		}
	}

	return v
}
