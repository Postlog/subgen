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
		target, ok := res.resolve(rule.Target)
		if !ok {
			if rule.Target.Kind != mihomo.PolicyInbound {
				slog.Error("render: rule target did not resolve",
					"subID", res.subID, "ruleType", rule.Type.String(), "kind", string(rule.Target.Kind))
			}

			continue
		}

		matcher, ok := renderMatcher(rule.Type, rule.Value, rule.ProviderID, rule.Conditions, res)
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

// renderMatcher renders a rule's (or a sub-condition's) matcher WITHOUT the target:
// "MATCH" for the catch-all, "TYPE,VALUE" for a plain matcher, "RULE-SET,<name>" for a
// rule-set (name resolved from the provider id), and "TYPE,((c1),(c2),…)" for a logical
// type — each child wrapped in its own parentheses (mihomo's nested-condition syntax).
// It returns false only when a RULE-SET's provider id does not resolve (a stored-config
// error, logged) so the whole rule is dropped. Sub-conditions never reference per-client
// inbounds (they are matchers, not policy targets), so they cause no per-subscriber drop.
func renderMatcher(typ mihomo.RuleType, value *string, providerID *int64, conds []mihomo.RuleCondition, res entityNameResolver) (string, bool) {
	switch {
	case typ.IsLogical():
		parts := make([]string, 0, len(conds))

		for _, c := range conds {
			s, ok := renderMatcher(c.Type, c.Value, c.ProviderID, c.Conditions, res)
			if !ok {
				return "", false
			}

			parts = append(parts, "("+s+")")
		}

		return typ.String() + ",(" + strings.Join(parts, ",") + ")", true
	case typ.TakesProvider(): // RULE-SET: payload is the provider name (from id)
		if providerID == nil {
			slog.Error("render: RULE-SET has no provider id", "subID", res.subID, "ruleType", typ.String())
			return "", false
		}

		name, ok := res.providerName[*providerID]
		if !ok {
			slog.Error("render: RULE-SET provider id did not resolve", "subID", res.subID, "providerID", *providerID)
			return "", false
		}

		return typ.String() + "," + name, true
	case value != nil:
		return typ.String() + "," + *value, true
	default: // MATCH (no payload)
		return typ.String(), true
	}
}
