package mihomo

// RuleProviderSource is how a rule-provider's content is supplied: an external upstream
// (a URL the client/mirror fetches) or authored in subgen (an operator-edited matcher
// list that subgen serves per-token as a classical text rule-provider). It is a typed
// value, not a free string.
type RuleProviderSource string

const (
	// RuleProviderExternal points at an upstream URL (optionally mirrored by subgen);
	// this is the legacy behavior and the default for rows predating the source column.
	RuleProviderExternal RuleProviderSource = "external"
	// RuleProviderAuthored carries an in-subgen matcher list (Matchers); subgen renders
	// it to classical text and serves it at /sub/mihomo/{token}/rules/{name}. Such a
	// provider is always behavior=classical/format=text and has no URL/mirror.
	RuleProviderAuthored RuleProviderSource = "authored"
)

// Valid reports whether s is a known source.
func (s RuleProviderSource) Valid() bool {
	return s == RuleProviderExternal || s == RuleProviderAuthored
}

// RuleProvider is a mihomo rule-provider (one row of the rule_providers table). Interval
// is the ruleset auto-update TTL (seconds), always rendered into the YAML so the mihomo
// core refreshes it while the tunnel is up. Source selects the content origin:
//   - external: Behavior/Format/URL describe the upstream. When Mirror is true, subgen
//     caches the upstream file and serves it, refreshing on MirrorInterval (seconds) —
//     independent of Interval.
//   - authored: Matchers carries the operator's list; Behavior/Format are fixed to
//     classical/text, URL/Mirror are empty, and subgen serves the rendered list per-token.
//
// ID is the surrogate id (mihomo_rule_providers.id) a RULE-SET rule references via
// RoutingRule.ProviderID. It is set on read; on the save path (inside a ConfigDraft)
// it is unset — not yet assigned. Name stays the provider's own attribute and the
// mihomo YAML key; the id never leaves the backend (the wire references by index).
//
// Matchers are target-less rule trees (leaf matchers + logical AND/OR/NOT, never MATCH,
// RULE-SET or SUB-RULE — mihomo rejects those inside a classical provider). They reuse
// RoutingRule with Target/ProviderID always nil: an authored matcher references no group
// or provider, so the draft/domain index-vs-id split does not apply to them.
type RuleProvider struct {
	ID             int64
	Name           string
	Source         RuleProviderSource
	Behavior       string
	Format         string
	Mirror         bool
	URL            string
	Interval       int
	MirrorInterval int
	Matchers       []RoutingRule
}

// RuleProviderBehaviors / RuleProviderFormats / RuleProviderSources are the mihomo options
// the admin schema offers for a rule-provider.
func RuleProviderBehaviors() []string { return []string{"domain", "ipcidr", "classical"} }
func RuleProviderFormats() []string   { return []string{"mrs", "yaml", "text"} }
func RuleProviderSources() []string {
	return []string{string(RuleProviderExternal), string(RuleProviderAuthored)}
}

// GeneratedKeys are the top-level mihomo YAML keys subgen owns; the operator's base YAML
// must not set them (render strips them, save rejects them). proxy-providers carries the
// auto-generated node list; sub-rules is reserved: subgen does not generate named sub-rule
// groups, but it owns the key so an operator cannot smuggle a sub-rules section into the
// base YAML and collide with generation.
func GeneratedKeys() []string {
	return []string{"proxies", "proxy-providers", "proxy-groups", "rules", "rule-providers", "sub-rules"}
}
