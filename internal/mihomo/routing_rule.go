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
	RuleSrcIPASN         RuleType = "SRC-IP-ASN"
	RuleSrcIPSuffix      RuleType = "SRC-IP-SUFFIX"
	RuleProcessName      RuleType = "PROCESS-NAME"
	RuleProcessNameWild  RuleType = "PROCESS-NAME-WILDCARD"
	RuleProcessPath      RuleType = "PROCESS-PATH"
	RuleProcessPathWild  RuleType = "PROCESS-PATH-WILDCARD"
	RuleProcessNameRegex RuleType = "PROCESS-NAME-REGEX"
	RuleNetwork          RuleType = "NETWORK"
	RuleDSCP             RuleType = "DSCP"
	RuleUID              RuleType = "UID"
	RuleRuleSet          RuleType = "RULE-SET"
	RuleMatch            RuleType = "MATCH"

	// Logical rules: their payload is a parenthesised list of sub-conditions
	// (RoutingRule.Conditions / RuleCondition), not a plain value. NOT takes exactly
	// one condition; AND/OR take two or more.
	RuleAnd RuleType = "AND"
	RuleOr  RuleType = "OR"
	RuleNot RuleType = "NOT"
)

// RuleTypeOptions are a rule type's admin-schema options: whether its payload is a
// rule-provider name (RULE-SET), whether the no-resolve option is meaningful, and
// whether it is a logical rule (AND/OR/NOT) whose payload is a list of sub-conditions
// instead of a plain value.
type RuleTypeOptions struct {
	TakesProvider     bool
	SupportsNoResolve bool
	Logical           bool
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
	RuleSrcIPASN:         {},
	RuleSrcIPSuffix:      {},
	RuleSrcPort:          {},
	RuleDstPort:          {},
	RuleInPort:           {},
	RuleInType:           {},
	RuleInName:           {},
	RuleInUser:           {},
	RuleProcessName:      {},
	RuleProcessNameWild:  {},
	RuleProcessPath:      {},
	RuleProcessPathWild:  {},
	RuleProcessNameRegex: {},
	RuleNetwork:          {},
	RuleDSCP:             {},
	RuleUID:              {},
	RuleRuleSet:          {TakesProvider: true, SupportsNoResolve: true},
	RuleMatch:            {},
	RuleAnd:              {Logical: true},
	RuleOr:               {Logical: true},
	RuleNot:              {Logical: true},
}

// RuleTypeCatalog returns the rule-type options map (the admin-schema source). The
// caller orders it (e.g. by name).
func RuleTypeCatalog() map[RuleType]RuleTypeOptions { return ruleTypes }

// Valid reports whether t is a known rule type.
func (t RuleType) Valid() bool { _, ok := ruleTypes[t]; return ok }

// IsMatch reports whether t is the catch-all MATCH (no payload).
func (t RuleType) IsMatch() bool { return t == RuleMatch }

// IsLogical reports whether t is a logical rule (AND/OR/NOT) — its payload is a list of
// sub-conditions, not a plain value, provider or no-resolve.
func (t RuleType) IsLogical() bool { return ruleTypes[t].Logical }

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
//
// Conditions is the sub-condition list of a logical rule (Type AND/OR/NOT); empty for
// every other type. A logical rule carries no Value/ProviderID/NoResolve — its matcher
// is the conditions, rendered as TYPE,((c1),(c2),…).
type RoutingRule struct {
	ID         int64
	Position   int
	Type       RuleType
	Value      *string
	ProviderID *int64
	NoResolve  *bool
	Target     PolicyRef
	Conditions []RuleCondition
}

// RuleCondition is one matcher inside a logical rule (AND/OR/NOT). It is a sub-condition,
// not a full rule: it carries no target and no no-resolve — mihomo parses sub-conditions
// without params, so no-resolve is meaningless here (rules/logic: ParseRulePayload with
// parseParams=false). Type is the matcher (any simple matcher, RULE-SET, or a nested
// logical type); Value is the plain payload (nil for RULE-SET and logical types);
// ProviderID is the rule-provider id for a RULE-SET condition (nil otherwise); Conditions
// is the nested sub-condition list when Type is itself logical (empty otherwise).
type RuleCondition struct {
	Type       RuleType
	Value      *string
	ProviderID *int64
	Conditions []RuleCondition
}
