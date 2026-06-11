package render

import (
	"log/slog"
	"strings"

	"github.com/postlog/subgen/internal/mihomo"
)

// buildRules renders the routing rules as mihomo rule lines for one subscriber. Line
// shape: TYPE[,VALUE],TARGET[,no-resolve]; MATCH has no value (MATCH,TARGET). For
// RULE-SET the value is the provider name, resolved from the rule's ProviderID.
//
// A rule whose target is a per-client inbound the subscriber lacks is the one EXPECTED
// miss — it is dropped silently (per-subscriber filtering). Any other unresolved ref (a
// group target, or a RULE-SET whose provider id is missing) means stored config that
// validation should have made impossible: it is logged at Error with context, then the
// rule is dropped so the subscription still renders.
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

		fields := []string{rule.Type.String()}

		switch {
		case rule.Type.TakesProvider(): // RULE-SET: payload is the provider name (from id)
			if rule.ProviderID == nil {
				slog.Error("render: RULE-SET rule has no provider id", "subID", res.subID, "ruleType", rule.Type.String())
				continue
			}

			name, ok := res.providerName[*rule.ProviderID]
			if !ok {
				slog.Error("render: RULE-SET provider id did not resolve", "subID", res.subID, "providerID", *rule.ProviderID)
				continue
			}

			fields = append(fields, name)
		case rule.Value != nil:
			fields = append(fields, *rule.Value)
		}

		fields = append(fields, target)
		if rule.NoResolve != nil && *rule.NoResolve {
			fields = append(fields, "no-resolve")
		}

		out = append(out, strings.Join(fields, ","))
	}

	return out
}
