package render

import (
	"strings"

	"github.com/postlog/subgen/internal/mihomo"
)

// buildRules renders the routing rules as mihomo rule lines for one subscriber. A
// rule whose target is a per-client inbound ref the subscriber lacks is dropped. Line
// shape: TYPE[,VALUE],TARGET[,no-resolve]; MATCH has no value (MATCH,TARGET). For
// RULE-SET the value is the provider name, resolved from the rule's ProviderID.
func buildRules(rules []mihomo.RoutingRule, res resolver) []string {
	out := make([]string, 0, len(rules))

	for _, rule := range rules {
		target, ok := res.resolve(rule.Target)
		if !ok {
			continue
		}

		fields := []string{rule.Type.String()}

		switch {
		case rule.Type.TakesProvider(): // RULE-SET: payload is the provider name (from id)
			if rule.ProviderID == nil {
				continue
			}

			name, ok := res.providerName[*rule.ProviderID]
			if !ok {
				continue
			}

			fields = append(fields, name)
		case rule.Value != "":
			fields = append(fields, rule.Value)
		}

		fields = append(fields, target)
		if rule.NoResolve {
			fields = append(fields, "no-resolve")
		}

		out = append(out, strings.Join(fields, ","))
	}

	return out
}
