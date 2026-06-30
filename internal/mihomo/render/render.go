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
//
// Token is the subscriber's subscription token: the auto node list (proxy-provider) and
// each authored rule-provider are served by subgen at per-token URLs, so render needs it
// to build those references. ProxiesInterval is the proxy-provider refresh TTL (seconds).
type Options struct {
	BaseYAML        string
	Rules           []mihomo.RoutingRule
	Groups          []mihomo.ProxyGroup
	Providers       []mihomo.RuleProvider
	PublicBase      string
	Token           string
	ProxiesInterval int
}

// Render produces the mihomo YAML bytes for one subscriber. The node list is NOT inlined:
// it is delivered as a proxy-provider pointing at /sub/mihomo/{token}/proxies (the core
// re-fetches it on ProxiesInterval while the tunnel is up); proxy-groups reference it via
// use:+filter. Authored rule-providers likewise point at per-token /rules/{name} URLs.
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

	res := newEntityNameResolver(sub, o.Groups, o.Providers)

	base["proxy-providers"] = proxyProviders(o)
	base["proxy-groups"] = buildGroups(o.Groups, res)
	base["rules"] = buildRules(o.Rules, res)

	if rp := ruleProviders(o); rp != nil {
		base["rule-providers"] = rp
	}

	return yaml.Marshal(base)
}

// proxyProviders renders the proxy-providers block: a single "proxies" http provider that
// points at subgen's per-token /proxies endpoint. The mihomo core re-fetches it on the
// configured interval, so node changes reach a connected client without a profile reload.
func proxyProviders(o Options) map[string]any {
	base := strings.TrimRight(o.PublicBase, "/")

	url := ""
	if base != "" {
		url = base + "/sub/mihomo/" + o.Token + "/proxies"
	}

	entry := map[string]any{
		"type": "http",
		"url":  url,
		"path": "./providers/proxies.yaml",
	}

	if o.ProxiesInterval > 0 {
		entry["interval"] = o.ProxiesInterval
	}

	return map[string]any{"proxies": entry}
}

// RenderProxiesPayload renders the body served at /sub/mihomo/{token}/proxies: the
// subscriber's node list as a proxy-provider payload (a YAML document with a top-level
// proxies: array). Reuses proxyToMap, the same per-node mapping as the inline form.
func RenderProxiesPayload(sub *entity.Subscriber) ([]byte, error) {
	proxies := make([]map[string]any, 0, len(sub.Proxies))
	for _, p := range sub.Proxies {
		proxies = append(proxies, proxyToMap(p))
	}

	return yaml.Marshal(map[string]any{"proxies": proxies})
}
