package render

import (
	"strings"

	"github.com/postlog/subgen/internal/mihomo"
)

// RenderAuthoredProvider renders an authored rule-provider's matcher tree as the body of a
// classical text rule-provider (the document served at /sub/mihomo/{token}/rules/{name}):
// one matcher per line, no target. A leaf is "TYPE,VALUE" (or "TYPE" when it carries none);
// a logical matcher is "TYPE,((c1),(c2),…)" — each child wrapped in its own parentheses,
// mihomo's nested-rule syntax. Authored matchers never contain MATCH/RULE-SET/SUB-RULE
// (validation forbids them), so no per-subscriber resolution is needed.
func RenderAuthoredProvider(matchers []mihomo.RoutingRule) []byte {
	lines := make([]string, 0, len(matchers))
	for _, m := range matchers {
		lines = append(lines, renderMatcherText(m))
	}

	if len(lines) == 0 {
		return []byte{}
	}

	return []byte(strings.Join(lines, "\n") + "\n")
}

// renderMatcherText renders one authored matcher (and its subtree) as a classical-rule line
// without a target: "TYPE,VALUE" for a leaf, "TYPE,((c1),(c2),…)" for a logical matcher.
func renderMatcherText(m mihomo.RoutingRule) string {
	if m.Type.IsLogical() {
		parts := make([]string, 0, len(m.Children))
		for _, c := range m.Children {
			parts = append(parts, "("+renderMatcherText(c)+")")
		}

		return m.Type.String() + ",(" + strings.Join(parts, ",") + ")"
	}

	if m.Value != nil {
		return m.Type.String() + "," + *m.Value
	}

	return m.Type.String()
}
