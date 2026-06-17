package mihomo

// Save-input "draft" types — the shape DecodeConfig produces and the repository's
// SaveMihomoConfig consumes, BEFORE persistence assigns ids. They are deliberately
// distinct from the domain types (RoutingRule/ProxyGroup): a draft references entities
// the same save creates (groups, providers) by their ARRAY INDEX, because their ids
// don't exist yet; the domain types (read back from the DB) reference them by real id.
// Keeping the two apart means no field ever carries "index on the way in, id on the way
// out". Inbounds are the exception — they exist independently of the config, so a draft
// references them by their real id (InboundID), never an index.
//
// Draft and domain never coexist in one graph: save is wire→ConfigDraft→DB, read is
// DB→domain. The index→id resolution lives inside SaveMihomoConfig (in local slices),
// not in any type.

// RefDraft is a PolicyRef at save time: a group ref carries the group's index in
// ConfigDraft.Groups; an inbound ref carries the real inbound id.
type RefDraft struct {
	Kind      PolicyKind
	InboundID *int64 // real node_inbounds.id when Kind==inbound
	GroupIdx  *int   // index into ConfigDraft.Groups when Kind==group
}

// Valid checks the ref is internally consistent (mirrors PolicyRef.Valid): a known kind,
// with InboundID iff inbound and GroupIdx iff group. Returns ErrBadRef when malformed —
// an error, like the other draft Valid() methods, so callers handle them uniformly.
func (r RefDraft) Valid() error {
	if !r.Kind.Valid() {
		return ErrBadRef
	}

	if (r.Kind == PolicyInbound) != (r.InboundID != nil) {
		return ErrBadRef
	}

	if (r.Kind == PolicyGroup) != (r.GroupIdx != nil) {
		return ErrBadRef
	}

	return nil
}

// RuleDraft is a routing rule at save time — the SAME recursive shape as the domain
// RoutingRule (no separate sub-condition type): a logical rule (AND/OR/NOT) carries its
// sub-rules in Children, every other type is a leaf with Children empty. ProviderIdx is
// the index into ConfigDraft.Providers for RULE-SET (nil otherwise); Value is the plain
// matcher payload (nil for RULE-SET, MATCH and the logical types). Target is OPTIONAL — a
// pointer: a top-level rule has one, a sub-rule (a Child of a logical rule) has none. The
// top-level/sub-rule distinction (target presence, MATCH placement, no-resolve) is fixed
// by ValidateRoutingRules; Valid() below is the position-independent per-type check.
type RuleDraft struct {
	Type        RuleType
	Value       *string
	ProviderIdx *int
	NoResolve   *bool
	Target      *RefDraft
	Children    []RuleDraft
}

// Valid checks the position-independent per-type field invariant: a logical rule carries
// neither value nor provider nor no-resolve (its matcher is the Children); MATCH carries
// neither value nor provider; RULE-SET carries a provider and no value; every other type
// carries a value and no provider. Children are allowed only on a logical type, and
// no-resolve only where the type supports it. The positional rules (a top-level rule has a
// target, a sub-rule does not; MATCH only at the top and last; index ranges; arity) are
// checked by ValidateRoutingRules, which knows a node's position and the counts.
func (r RuleDraft) Valid() error {
	if !r.Type.Valid() {
		return ErrUnknownRuleType
	}

	switch {
	case r.Type.IsLogical():
		// A logical rule carries no value/provider. A meaningful (true) no-resolve is
		// caught by the SupportsNoResolve check below; an explicit false is harmless
		// (clients may send noResolve:false unconditionally), so it is NOT rejected here.
		if r.Value != nil || r.ProviderIdx != nil {
			return ErrRulePayloadNotAllowed
		}
	case r.Type.IsMatch():
		if r.Value != nil || r.ProviderIdx != nil {
			return ErrRulePayloadNotAllowed
		}
	case r.Type.TakesProvider():
		if r.Value != nil {
			return ErrRulePayloadNotAllowed
		}

		if r.ProviderIdx == nil {
			return ErrProviderRefRange
		}
	default:
		if r.ProviderIdx != nil {
			return ErrRulePayloadNotAllowed
		}

		if r.Value == nil || *r.Value == "" {
			return ErrRuleValueRequired
		}
	}

	if len(r.Children) != 0 && !r.Type.IsLogical() {
		return ErrChildrenNotAllowed
	}

	if r.NoResolve != nil && *r.NoResolve && !r.Type.SupportsNoResolve() {
		return ErrNoResolveUnsupported
	}

	return nil
}

// GroupDraft is a proxy-group at save time: no id yet, members reference by index.
// Interval/Tolerance/Lazy are optional → pointers (nil = not set / not applicable).
type GroupDraft struct {
	Name      string
	Type      ProxyGroupType
	URL       string
	Interval  *int
	Tolerance *int
	Lazy      *bool
	Members   []RefDraft
}

// Valid checks the intra-group invariant: a non-empty name, a known type, at least one
// member, and that the health-check fields are present only for types that use them
// (url/interval/lazy for the health-check types; tolerance for url-test). Name
// uniqueness, the member-ref ranges and the group cycle are checked by ValidateProxyGroups.
func (g GroupDraft) Valid() error {
	if g.Name == "" {
		return ErrGroupNameEmpty
	}

	if !g.Type.Valid() {
		return ErrGroupUnknownType
	}

	if len(g.Members) == 0 {
		return ErrGroupNoMembers
	}

	if !g.Type.UsesHealthCheck() && (g.URL != "" || g.Interval != nil || g.Lazy != nil) {
		return ErrGroupFieldNotAllowed
	}

	if !g.Type.UsesTolerance() && g.Tolerance != nil {
		return ErrGroupFieldNotAllowed
	}

	return nil
}

// ConfigDraft is the whole decoded mihomo config ready to save. Providers reuse the
// domain RuleProvider — its ID is simply unset here (not yet assigned), which is an
// "absent" value, not a second meaning.
type ConfigDraft struct {
	Groups    []GroupDraft
	Rules     []RuleDraft
	Providers []RuleProvider
	BaseYAML  string
	Profile   Profile
}
