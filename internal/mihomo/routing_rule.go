package mihomo

// RuleType is a mihomo (Clash.Meta) rule matcher. It is a typed value, not a free
// string — the admin UI offers exactly these and the renderer emits them verbatim.
type RuleType string

const (
	RuleDomain           RuleType = "DOMAIN"
	RuleDomainSuffix     RuleType = "DOMAIN-SUFFIX"
	RuleDomainKeyword    RuleType = "DOMAIN-KEYWORD"
	RuleDomainRegex      RuleType = "DOMAIN-REGEX"
	RuleDomainWildcard   RuleType = "DOMAIN-WILDCARD"
	RuleGeosite          RuleType = "GEOSITE"
	RuleIPCIDR           RuleType = "IP-CIDR"
	RuleIPCIDR6          RuleType = "IP-CIDR6"
	RuleIPSuffix         RuleType = "IP-SUFFIX"
	RuleIPASN            RuleType = "IP-ASN"
	RuleGeoIP            RuleType = "GEOIP"
	RuleSrcGeoIP         RuleType = "SRC-GEOIP"
	RuleSrcIPCIDR        RuleType = "SRC-IP-CIDR"
	RuleSrcPort          RuleType = "SRC-PORT"
	RuleDstPort          RuleType = "DST-PORT"
	RuleInPort           RuleType = "IN-PORT"
	RuleInType           RuleType = "IN-TYPE"
	RuleInName           RuleType = "IN-NAME"
	RuleInUser           RuleType = "IN-USER"
	RuleProcessName      RuleType = "PROCESS-NAME"
	RuleProcessPath      RuleType = "PROCESS-PATH"
	RuleProcessNameRegex RuleType = "PROCESS-NAME-REGEX"
	RuleNetwork          RuleType = "NETWORK"
	RuleDSCP             RuleType = "DSCP"
	RuleUID              RuleType = "UID"
	RuleRuleSet          RuleType = "RULE-SET"
	RuleMatch            RuleType = "MATCH"
)

// RuleTypeOptions are a rule type's admin-schema options: whether its payload is a
// rule-provider name (RULE-SET) and whether the no-resolve option is meaningful.
type RuleTypeOptions struct {
	TakesProvider     bool
	SupportsNoResolve bool
}

// ruleTypes is the known-type registry (single source for validity, options and the
// admin schema).
var ruleTypes = map[RuleType]RuleTypeOptions{
	RuleDomain:           {},
	RuleDomainSuffix:     {},
	RuleDomainKeyword:    {},
	RuleDomainRegex:      {},
	RuleDomainWildcard:   {},
	RuleGeosite:          {},
	RuleIPCIDR:           {SupportsNoResolve: true},
	RuleIPCIDR6:          {SupportsNoResolve: true},
	RuleIPSuffix:         {SupportsNoResolve: true},
	RuleIPASN:            {SupportsNoResolve: true},
	RuleGeoIP:            {SupportsNoResolve: true},
	RuleSrcGeoIP:         {},
	RuleSrcIPCIDR:        {},
	RuleSrcPort:          {},
	RuleDstPort:          {},
	RuleInPort:           {},
	RuleInType:           {},
	RuleInName:           {},
	RuleInUser:           {},
	RuleProcessName:      {},
	RuleProcessPath:      {},
	RuleProcessNameRegex: {},
	RuleNetwork:          {},
	RuleDSCP:             {},
	RuleUID:              {},
	RuleRuleSet:          {TakesProvider: true, SupportsNoResolve: true},
	RuleMatch:            {},
}

// RuleTypeCatalog returns the rule-type options map (the admin-schema source). The
// caller orders it (e.g. by name).
func RuleTypeCatalog() map[RuleType]RuleTypeOptions { return ruleTypes }

// Valid reports whether t is a known rule type.
func (t RuleType) Valid() bool { _, ok := ruleTypes[t]; return ok }

// IsMatch reports whether t is the catch-all MATCH (no payload).
func (t RuleType) IsMatch() bool { return t == RuleMatch }

// TakesProvider reports whether the rule's payload is a rule-provider name (RULE-SET).
func (t RuleType) TakesProvider() bool { return ruleTypes[t].TakesProvider }

// SupportsNoResolve reports whether the no-resolve option is meaningful for t (IP
// matchers and RULE-SET).
func (t RuleType) SupportsNoResolve() bool { return ruleTypes[t].SupportsNoResolve }

// String returns the wire value (used as the rule line's first field).
func (t RuleType) String() string { return string(t) }

// RoutingRule is one ordered mihomo rule with a typed target (PolicyRef). Value is the
// plain matcher payload — optional (pointer): nil for RULE-SET and MATCH, set for every
// other type. NoResolve is the optional no-resolve option (pointer; nil/false = off).
// ProviderID is the rule-provider this rule points at by id (RULE-SET only); nil for
// every other type. The provider name is resolved from the id at render — the rule never
// carries the name as a string (that was the old dirty Value overload).
type RoutingRule struct {
	ID         int64
	Position   int
	Type       RuleType
	Value      *string
	ProviderID *int64
	NoResolve  *bool
	Target     PolicyRef
}
