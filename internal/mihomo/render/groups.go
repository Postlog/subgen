package render

import (
	"log/slog"
	"regexp"
	"strings"

	"github.com/postlog/subgen/internal/mihomo"
)

// buildGroups renders the operator-defined proxy-groups for one subscriber. Members are
// resolved via the per-client resolver and split: inbound members become proxies pulled
// from the auto "proxies" provider (use: [proxies] + a filter regex matching exactly their
// names), while built-in policies (DIRECT/REJECT/…) and group references stay inline by
// name in proxies:. An inbound a subscriber lacks is the expected miss and is dropped
// silently; any other unresolved member is logged at Error (invalid stored config). A group
// left with no members at all falls back to DIRECT (mihomo requires ≥1). interval/tolerance/
// lazy are emitted when set (non-nil) — a nil field means "not set / not applicable".
func buildGroups(groups []mihomo.ProxyGroup, res entityNameResolver) []map[string]any {
	out := make([]map[string]any, 0, len(groups))

	for _, g := range groups {
		var (
			static  []string // built-in policies + group refs, inline by name
			inbound []string // resolved proxy names, pulled from the "proxies" provider
		)

		for _, m := range g.Members {
			name, ok := res.resolve(m)
			if !ok {
				if m.Kind != mihomo.PolicyInbound {
					slog.Error("render: group member did not resolve", "subID", res.subID, "group", g.Name, "kind", string(m.Kind))
				}

				continue
			}

			if m.Kind == mihomo.PolicyInbound {
				inbound = append(inbound, name)
			} else {
				static = append(static, name)
			}
		}

		gm := map[string]any{
			"name": g.Name,
			"type": g.Type.String(),
		}

		switch {
		case len(inbound) > 0:
			// Pull the inbound proxies from the auto provider; keep any built-in/group
			// members inline. A group may carry both proxies: and use:.
			gm["use"] = []string{"proxies"}
			gm["filter"] = proxyFilterRegex(inbound)

			if len(static) > 0 {
				gm["proxies"] = static
			}
		case len(static) > 0:
			gm["proxies"] = static
		default:
			// Nothing resolved (e.g. only inbounds the subscriber lacks) → DIRECT.
			gm["proxies"] = []string{"DIRECT"}
		}

		if g.Type.UsesHealthCheck() {
			if g.URL != "" {
				gm["url"] = g.URL
			}

			if g.Interval != nil {
				gm["interval"] = *g.Interval
			}

			if g.Type == mihomo.GroupURLTest && g.Tolerance != nil {
				gm["tolerance"] = *g.Tolerance
			}

			if g.Lazy != nil {
				gm["lazy"] = *g.Lazy
			}
		}

		out = append(out, gm)
	}

	return out
}

// proxyFilterRegex builds an anchored alternation that matches exactly the given proxy
// names — the mihomo proxy-group filter (a regex over proxy names from use:). Each name is
// regexp-escaped: node/inbound labels carry arbitrary operator text (emoji, regex
// metacharacters), so a raw name would mis-match.
func proxyFilterRegex(names []string) string {
	escaped := make([]string, len(names))
	for i, n := range names {
		escaped[i] = regexp.QuoteMeta(n)
	}

	return "^(" + strings.Join(escaped, "|") + ")$"
}
