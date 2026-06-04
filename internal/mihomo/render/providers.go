package render

import "strings"

// ruleProviders renders the mihomo rule-providers block. Mirrored providers point
// at subgen's own /rules/<name><ext> endpoint; others use the upstream URL.
func ruleProviders(o Options) map[string]any {
	if len(o.Providers) == 0 {
		return nil
	}

	base := strings.TrimRight(o.PublicBase, "/")
	out := map[string]any{}

	for _, rp := range o.Providers {
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
