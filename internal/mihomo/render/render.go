// Package render turns a subscriber + operator config into a mihomo YAML config:
// the operator's editable "base" YAML (dns, sniffer, knobs…) with the generated
// sections (proxies, proxy-groups, rules, rule-providers) injected.
package render

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/mihomo"
)

// generated lists the top-level keys subgen owns; they are stripped from the base
// YAML and replaced with freshly generated content (mihomo is the single source).
var generated = mihomo.GeneratedKeys()

// Options is the operator-configured mihomo data render needs, assembled from the
// store by the caller. render has no dependency on the config package. Proxy-groups
// and routing rules are structured (typed PolicyRefs), resolved per-subscriber.
type Options struct {
	BaseYAML   string
	Rules      []mihomo.RoutingRule
	Groups     []mihomo.ProxyGroup
	Providers  []mihomo.RuleProvider
	PublicBase string
}

// Render produces the mihomo YAML bytes for one subscriber.
func Render(sub *entity.Subscriber, o Options) ([]byte, error) {
	base := map[string]any{}
	if strings.TrimSpace(o.BaseYAML) != "" {
		if err := yaml.Unmarshal([]byte(o.BaseYAML), &base); err != nil {
			return nil, fmt.Errorf("base config: %w", err)
		}

		if base == nil {
			base = map[string]any{}
		}
	}

	for _, k := range generated {
		delete(base, k)
	}

	proxies := make([]map[string]any, 0, len(sub.Proxies))
	for _, p := range sub.Proxies {
		proxies = append(proxies, proxyToMap(p))
	}

	res := newEntityNameResolver(sub, o.Groups, o.Providers)

	base["proxies"] = proxies
	base["proxy-groups"] = buildGroups(o.Groups, res)
	base["rules"] = buildRules(o.Rules, res)

	if rp := ruleProviders(o); rp != nil {
		base["rule-providers"] = rp
	}

	return yaml.Marshal(base)
}
