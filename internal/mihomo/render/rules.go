package render

import (
	"log/slog"
	"strings"

	"github.com/postlog/subgen/internal/mihomo"
)

// buildRules renders the routing rules as mihomo rule lines for one subscriber. Line
// shape: MATCHER,TARGET[,no-resolve], where MATCHER is TYPE[,VALUE] for a plain rule,
// RULE-SET,<provider name> for a rule-set, or TYPE,((c1),(c2),…) for a logical rule
// (AND/OR/NOT). For RULE-SET the value is the provider name, resolved from the id.
//
// A rule whose target is a per-client inbound the subscriber lacks is the one EXPECTED
// miss — it is dropped silently (per-subscriber filtering). Any other unresolved ref (a
// group target, or a RULE-SET whose provider id is missing — at the top level or inside a
// logical rule) means stored config that validation should have made impossible: it is
// logged at Error with context, then the rule is dropped so the subscription still
// renders.
func buildRules(rules []mihomo.RoutingRule, res entityNameResolver) []string {
	out := make([]string, 0, len(rules))

	for _, rule := range rules {
		// Only top-level rules are iterated here; a top-level rule always has a target
		// (validation guarantees it). A nil target is stored corruption — drop and log.
		if rule.Target == nil {
			slog.Error("render: top-level rule has no target", "subID", res.subID, "ruleType", rule.Type.String())
			continue
		}

		target, ok := res.resolve(*rule.Target)
		if !ok {
			if rule.Target.Kind != mihomo.PolicyInbound {
				slog.Error("render: rule target did not resolve",
					"subID", res.subID, "ruleType", rule.Type.String(), "kind", string(rule.Target.Kind))
			}

			continue
		}

		matcher, ok := renderMatcher(rule, res)
		if !ok {
			continue // a RULE-SET provider did not resolve (logged inside) → drop the rule
		}

		line := matcher + "," + target
		if rule.NoResolve != nil && *rule.NoResolve {
			line += ",no-resolve"
		}

		out = append(out, line)
	}

	return out
}

// renderMatcher renders a rule's (or a sub-rule's) matcher WITHOUT the target: "MATCH"
// for the catch-all, "TYPE,VALUE" for a plain matcher, "RULE-SET,<name>" for a rule-set
// (name resolved from the provider id), and "TYPE,((c1),(c2),…)" for a logical type —
// each child wrapped in its own parentheses (mihomo's nested-rule syntax). It returns
// false only when a RULE-SET's provider id does not resolve (a stored-config error,
// logged) so the whole rule is dropped. Sub-rules never reference per-client inbounds
// (they are matchers, not policy targets), so they cause no per-subscriber drop.
func renderMatcher(r mihomo.RoutingRule, res entityNameResolver) (string, bool) {
	switch {
	case r.Type.IsLogical():
		parts := make([]string, 0, len(r.Children))

		for _, c := range r.Children {
			s, ok := renderMatcher(c, res)
			if !ok {
				return "", false
			}

			parts = append(parts, "("+s+")")
		}

		return r.Type.String() + ",(" + strings.Join(parts, ",") + ")", true
	case r.Type.TakesProvider(): // RULE-SET: payload is the provider name (from id)
		if r.ProviderID == nil {
			slog.Error("render: RULE-SET has no provider id", "subID", res.subID, "ruleType", r.Type.String())
			return "", false
		}

		name, ok := res.providerName[*r.ProviderID]
		if !ok {
			slog.Error("render: RULE-SET provider id did not resolve", "subID", res.subID, "providerID", *r.ProviderID)
			return "", false
		}

		return r.Type.String() + "," + name, true
	case r.Value != nil:
		return r.Type.String() + "," + *r.Value, true
	default: // MATCH (no payload)
		return r.Type.String(), true
	}
}
