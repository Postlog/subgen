package mihomo

import "errors"

// Domain sentinel errors for decoding/validating the operator-edited mihomo config.
// They carry no human-readable text — the handler layer maps them to user-facing
// messages (see web.UserMessage). Lower layers return these; never interpolate a
// name/value into the error text.
var (
	// Proxy-groups.
	ErrGroupNameEmpty       = errors.New("proxy-group name is empty")
	ErrGroupNameTaken       = errors.New("proxy-group name is duplicated")
	ErrGroupUnknownType     = errors.New("unknown proxy-group type")
	ErrGroupNoMembers       = errors.New("proxy-group has no members")
	ErrGroupCycle           = errors.New("proxy-groups form a reference cycle")
	ErrGroupFieldNotAllowed = errors.New("proxy-group sets a field its type does not use")

	// Policy refs (a rule target / group member).
	ErrBadRef        = errors.New("malformed policy ref")
	ErrGroupRefRange = errors.New("policy ref points at a non-existent group")

	// Routing rules.
	ErrUnknownRuleType       = errors.New("unknown rule type")
	ErrMatchNotLast          = errors.New("MATCH rule must be last")
	ErrRuleValueRequired     = errors.New("rule needs a value")
	ErrRulePayloadNotAllowed = errors.New("rule type does not take this payload")
	ErrNoResolveUnsupported  = errors.New("rule type does not support no-resolve")
	ErrProviderRefRange      = errors.New("RULE-SET references a non-existent provider")

	// Logical rules (AND/OR/NOT) and their sub-rules (Children).
	ErrChildrenNotAllowed = errors.New("non-logical rule cannot carry sub-rules")
	ErrNotArity           = errors.New("NOT must contain exactly one sub-rule")
	ErrLogicalArity       = errors.New("AND/OR must contain at least two sub-rules")
	ErrMatchChild         = errors.New("MATCH cannot be a sub-rule")
	ErrTargetRequired     = errors.New("rule has no target")
	ErrChildTarget        = errors.New("a sub-rule cannot carry a target")

	// Rule-providers. (Name uniqueness is enforced by the DB UNIQUE(config_id,name),
	// translated to entity.ErrRuleProviderNameTaken in the repository — not here.)
	ErrProviderNameEmpty   = errors.New("rule-provider name is empty")
	ErrProviderBadBehavior = errors.New("unknown rule-provider behavior")
	ErrProviderBadFormat   = errors.New("unknown rule-provider format")
	ErrProviderURLEmpty    = errors.New("rule-provider url is empty")

	// Base YAML.
	ErrBaseYAMLInvalid     = errors.New("base YAML is invalid")
	ErrGeneratedKeyPresent = errors.New("base YAML carries a generated section")

	// Subscription profile.
	ErrProfileTitleEmpty            = errors.New("profile title is empty")
	ErrProfileFilenameEmpty         = errors.New("profile filename is empty")
	ErrProfileFilenameInvalid       = errors.New("profile filename has path separators or control characters")
	ErrProfileUpdateIntervalInvalid = errors.New("profile update interval must be a positive number of hours")
)
