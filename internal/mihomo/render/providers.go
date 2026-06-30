package render

import (
	neturl "net/url"
	"strings"

	"github.com/postlog/subgen/internal/mihomo"
)

// ruleProviders renders the mihomo rule-providers block. An authored provider points at
// subgen's per-token /sub/mihomo/{token}/rules/{name} endpoint (classical text subgen
// serves itself); a mirrored external provider at subgen's /rules/<name><ext>; a plain
// external provider at its upstream URL. The core refreshes each on its interval.
func ruleProviders(o Options) map[string]any {
	if len(o.Providers) == 0 {
		return nil
	}

	base := strings.TrimRight(o.PublicBase, "/")
	out := map[string]any{}

	for _, rp := range o.Providers {
		if rp.Source == mihomo.RuleProviderAuthored {
			out[rp.Name] = authoredProviderEntry(rp, base, o.Token)
			continue
		}

		ext := ruleSetExt(rp.Format)
		url := rp.URL

		if rp.Mirror && base != "" {
			url = base + "/rules/" + rp.Name + ext
		}

		entry := map[string]any{
			"type":     "http",
			"behavior": rp.Behavior,
			"url":      url,
			"path":     "./ruleset/" + rp.Name + ext,
		}
		if rp.Format != "" {
			entry["format"] = rp.Format
		}

		if rp.Interval > 0 {
			entry["interval"] = rp.Interval
		}

		out[rp.Name] = entry
	}

	return out
}

// authoredProviderEntry renders one authored rule-provider: a classical/text http provider
// whose url is subgen's per-token /rules/{name} endpoint (name path-escaped). The core
// re-fetches it on the provider's interval, so edits reach a connected client live.
func authoredProviderEntry(rp mihomo.RuleProvider, base, token string) map[string]any {
	url := ""
	if base != "" {
		url = base + "/sub/mihomo/" + token + "/rules/" + neturl.PathEscape(rp.Name) + ".txt"
	}

	entry := map[string]any{
		"type":     "http",
		"behavior": "classical",
		"format":   "text",
		"url":      url,
		"path":     "./ruleset/" + rp.Name + ".txt",
	}

	if rp.Interval > 0 {
		entry["interval"] = rp.Interval
	}

	return entry
}

// ruleSetExt maps a rule-provider format to the file extension used by the mirror.
func ruleSetExt(format string) string {
	switch strings.ToLower(format) {
	case "mrs":
		return ".mrs"
	case "yaml":
		return ".yaml"
	default:
		return ".txt"
	}
}
