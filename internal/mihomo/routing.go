package mihomo

// RuleProvider is a mihomo rule-provider source (one row of the rule_providers
// table). Interval is the mihomo client's ruleset auto-update TTL (seconds), always
// rendered into the YAML. When Mirror is true, subgen also caches the upstream file
// and serves it, refreshing on MirrorInterval (seconds) — independent of Interval.
//
// ID is the surrogate id (mihomo_rule_providers.id) a RULE-SET rule references via
// RoutingRule.ProviderID. It is set on read; on the save path (inside a ConfigDraft)
// it is unset — not yet assigned. Name stays the provider's own attribute and the
// mihomo YAML key; the id never leaves the backend (the wire references by index).
type RuleProvider struct {
	ID             int64
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

// GeneratedKeys are the top-level mihomo YAML keys subgen owns; the operator's base YAML
// must not set them (render strips them, save rejects them). sub-rules is reserved: subgen
// does not generate named sub-rule groups, but it owns the key so an operator cannot smuggle
// a sub-rules section into the base YAML and collide with generation.
func GeneratedKeys() []string {
	return []string{"proxies", "proxy-groups", "rules", "rule-providers", "sub-rules"}
}
