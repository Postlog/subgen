package mihomo

import "errors"

// Domain sentinel errors for decoding/validating the operator-edited mihomo config.
// They carry no human-readable text — the handler layer maps them to user-facing
// messages (see web.UserMessage). Lower layers return these; never interpolate a
// name/value into the error text.
var (
	// Proxy-groups.
	ErrGroupNameEmpty   = errors.New("proxy-group name is empty")
	ErrGroupNameTaken   = errors.New("proxy-group name is duplicated")
	ErrGroupUnknownType = errors.New("unknown proxy-group type")
	ErrGroupNoMembers   = errors.New("proxy-group has no members")
	ErrGroupCycle       = errors.New("proxy-groups form a reference cycle")

	// Policy refs (a rule target / group member).
	ErrBadRef        = errors.New("malformed policy ref")
	ErrGroupRefRange = errors.New("policy ref points at a non-existent group")

	// Routing rules.
	ErrUnknownRuleType   = errors.New("unknown rule type")
	ErrMatchNotLast      = errors.New("MATCH rule must be last")
	ErrRuleValueRequired = errors.New("rule needs a value")

	// Rule-providers. (Name uniqueness is enforced by the DB PK, translated to
	// entity.ErrRuleProviderNameTaken in the repository — not validated here.)
	ErrProviderNameEmpty      = errors.New("rule-provider name is empty")
	ErrProviderBadBehavior    = errors.New("unknown rule-provider behavior")
	ErrProviderBadFormat      = errors.New("unknown rule-provider format")
	ErrProviderURLEmpty       = errors.New("rule-provider url is empty")
	ErrRuleSetUnknownProvider = errors.New("RULE-SET references an unknown provider")

	// Base YAML.
	ErrBaseYAMLInvalid     = errors.New("base YAML is invalid")
	ErrGeneratedKeyPresent = errors.New("base YAML carries a generated section")
)
