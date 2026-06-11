package render

import "github.com/postlog/subgen/internal/mihomo"

// buildGroups renders the operator-defined proxy-groups for one subscriber. Each
// group's members are resolved via the per-client resolver; members that resolve to
// nothing (an inbound the subscriber lacks) are dropped. A
// group left with no members falls back to DIRECT (mihomo requires ≥1).
func buildGroups(groups []mihomo.ProxyGroup, res resolver) []map[string]any {
	out := make([]map[string]any, 0, len(groups))

	for _, g := range groups {
		members := make([]string, 0, len(g.Members))
		for _, m := range g.Members {
			if name, ok := res.resolve(m); ok {
				members = append(members, name)
			}
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

			if g.Interval != nil && *g.Interval > 0 {
				gm["interval"] = *g.Interval
			}

			if g.Type == mihomo.GroupURLTest && g.Tolerance != nil && *g.Tolerance > 0 {
				gm["tolerance"] = *g.Tolerance
			}

			if g.Lazy != nil && *g.Lazy {
				gm["lazy"] = true
			}
		}

		out = append(out, gm)
	}

	return out
}
