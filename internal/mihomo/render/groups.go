package render

import (
	"log/slog"

	"github.com/postlog/subgen/internal/mihomo"
)

// buildGroups renders the operator-defined proxy-groups for one subscriber. Each group's
// members are resolved via the per-client resolver; an inbound a subscriber lacks is the
// expected miss and is dropped silently, while any other unresolved member is logged at
// Error (invalid stored config). A group left with no members falls back to DIRECT
// (mihomo requires ≥1). interval/tolerance/lazy are emitted when set (non-nil) — a nil
// field means "not set / not applicable to the type" and is omitted.
func buildGroups(groups []mihomo.ProxyGroup, res entityNameResolver) []map[string]any {
	out := make([]map[string]any, 0, len(groups))

	for _, g := range groups {
		members := make([]string, 0, len(g.Members))
		for _, m := range g.Members {
			name, ok := res.resolve(m)
			if !ok {
				if m.Kind != mihomo.PolicyInbound {
					slog.Error("render: group member did not resolve", "subID", res.subID, "group", g.Name, "kind", string(m.Kind))
				}

				continue
			}

			members = append(members, name)
		}

		if len(members) == 0 {
			members = []string{"DIRECT"}
		}

		gm := map[string]any{
			"name":    g.Name,
			"type":    g.Type.String(),
			"proxies": members,
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
