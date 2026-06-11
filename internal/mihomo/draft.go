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

// Valid reports whether the ref is internally consistent (mirrors PolicyRef.Valid).
func (r RefDraft) Valid() bool {
	if !r.Kind.Valid() {
		return false
	}

	if (r.Kind == PolicyInbound) != (r.InboundID != nil) {
		return false
	}

	if (r.Kind == PolicyGroup) != (r.GroupIdx != nil) {
		return false
	}

	return true
}

// RuleDraft is a routing rule at save time. ProviderIdx is the index into
// ConfigDraft.Providers for RULE-SET (nil otherwise); Value is the plain matcher
// payload ("" for RULE-SET and MATCH).
type RuleDraft struct {
	Type        RuleType
	Value       string
	ProviderIdx *int
	NoResolve   bool
	Target      RefDraft
}

// GroupDraft is a proxy-group at save time: no id yet, members reference by index.
type GroupDraft struct {
	Name      string
	Type      ProxyGroupType
	URL       string
	Interval  int
	Tolerance int
	Lazy      bool
	Members   []RefDraft
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
