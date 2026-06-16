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

	// Logical rules: their payload is a parenthesised list of sub-rules
	// (RoutingRule.Children — the same recursive type), not a plain value. NOT takes
	// exactly one sub-rule; AND/OR take two or more.
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

// RoutingRule is one mihomo rule. It is recursive: a logical rule (Type AND/OR/NOT)
// carries its sub-rules in Children (rendered as TYPE,((c1),(c2),…)); every other type is
// a leaf with Children empty. Value is the plain matcher payload — optional (pointer): nil
// for RULE-SET, MATCH and the logical types, set for every other type. ProviderID is the
// rule-provider this rule points at by id (RULE-SET only). NoResolve is the optional
// no-resolve option (pointer; nil/false = off); only a top-level rule carries it.
//
// Target is the typed routing target — a pointer because it is OPTIONAL: a top-level rule
// always has one, but a Child (a sub-rule of a logical rule) has none (it is a matcher,
// not a routing decision). A child also carries no no-resolve — mihomo parses sub-rules
// without params (rules/logic: ParseRulePayload with parseParams=false). The same type
// describes a rule and a sub-rule; the difference is the absent Target, fixed by validation.
type RoutingRule struct {
	ID         int64
	Position   int
	Type       RuleType
	Value      *string
	ProviderID *int64
	NoResolve  *bool
	Target     *PolicyRef
	Children   []RoutingRule
}
