package mihomo

// RuleProvider is a mihomo rule-provider source (one row of the rule_providers
// table). Interval is the mihomo client's ruleset auto-update TTL (seconds), always
// rendered into the YAML. When Mirror is true, subgen also caches the upstream file
// and serves it, refreshing on MirrorInterval (seconds) — independent of Interval.
type RuleProvider struct {
	Name           string
	Behavior       string
	Format         string
	Mirror         bool
	URL            string
	Interval       int
	MirrorInterval int
}

// RuleProviderBehaviors / RuleProviderFormats are the mihomo options the admin schema
// offers for a rule-provider.
func RuleProviderBehaviors() []string { return []string{"domain", "ipcidr", "classical"} }
func RuleProviderFormats() []string   { return []string{"mrs", "yaml", "text"} }

// GeneratedKeys are the top-level mihomo YAML keys subgen generates and injects; the
// operator's base YAML must not set them (render strips them, save rejects them).
func GeneratedKeys() []string { return []string{"proxies", "proxy-groups", "rules", "rule-providers"} }
