package mihomo

// PolicyKind is the kind of a routing target / proxy-group member: a built-in mihomo
// policy, an inbound (by id), or another proxy-group (by id). Code branches on these
// constants, never on a rendered name.
type PolicyKind string

const (
	// Built-in mihomo policies (client-independent).
	PolicyDirect       PolicyKind = "direct"
	PolicyReject       PolicyKind = "reject"
	PolicyRejectDrop   PolicyKind = "reject-drop"
	PolicyRejectNoDrop PolicyKind = "reject-no-drop"
	PolicyPass         PolicyKind = "pass"
	// References resolved against the subscriber at render (dropped when absent).
	PolicyInbound PolicyKind = "inbound" // a node inbound (PolicyRef.InboundID)
	PolicyGroup   PolicyKind = "group"   // a reference to another proxy-group (PolicyRef.GroupID)
)

// Valid reports whether k is a known policy kind.
func (k PolicyKind) Valid() bool {
	switch k {
	case PolicyDirect, PolicyReject, PolicyRejectDrop, PolicyRejectNoDrop, PolicyPass,
		PolicyInbound, PolicyGroup:
		return true
	default:
		return false
	}
}

// BuiltinPolicyKinds returns the client-independent policy kinds (DIRECT/REJECT/…/
// PASS) for the admin schema. inbound/group are dynamic (resolved per-subscriber).
func BuiltinPolicyKinds() []PolicyKind {
	return []PolicyKind{PolicyDirect, PolicyReject, PolicyRejectDrop, PolicyRejectNoDrop, PolicyPass}
}

// PolicyCategory groups what a PolicyRef may point at, for the admin picker: the
// fixed built-in policies (actions), the node inbounds, or the other proxy-groups.
// The schema declares, per rule/group type, which categories are allowed — the
// frontend renders the picker purely from that.
type PolicyCategory string

const (
	CategoryActions  PolicyCategory = "actions"  // built-in policies (fixed list)
	CategoryInbounds PolicyCategory = "inbounds" // node inbounds (runtime)
	CategoryGroups   PolicyCategory = "groups"   // other proxy-groups (runtime)
)

// PolicyCategories returns the reference categories a routing rule's target / a
// proxy-group member may point at (currently the same set for both).
func PolicyCategories() []PolicyCategory {
	return []PolicyCategory{CategoryActions, CategoryInbounds, CategoryGroups}
}

// PolicyRef is a typed pointer to a routing target / group member: a built-in policy,
// a node inbound, or another proxy-group. Exactly one mode: a valid Kind, with
// InboundID iff Kind==inbound, GroupID iff Kind==group.
type PolicyRef struct {
	Kind      PolicyKind
	InboundID *int64 // node_inbounds.id when Kind==inbound
	GroupID   *int64 // proxy_groups.id when Kind==group
}

// Valid reports whether the ref is internally consistent.
func (r PolicyRef) Valid() bool {
	if !r.Kind.Valid() {
		return false
	}

	if (r.Kind == PolicyInbound) != (r.InboundID != nil) {
		return false
	}

	if (r.Kind == PolicyGroup) != (r.GroupID != nil) {
		return false
	}

	return true
}
