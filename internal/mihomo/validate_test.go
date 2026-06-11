package mihomo

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func i64(v int64) *int64  { return &v }
func ip(v int) *int       { return &v }
func sp(v string) *string { return &v }

func TestValidateBaseYAML(t *testing.T) {
	tt := []struct {
		name string
		base string
		err  error
	}{
		{name: "success.valid", base: "mode: rule\ndns: {}"},
		{name: "error.invalid_yaml", base: "foo: [unclosed", err: ErrBaseYAMLInvalid},
		{name: "error.generated_section", base: "rules: []", err: ErrGeneratedKeyPresent},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.ErrorIs(t, ValidateBaseYAML(tc.base), tc.err)
		})
	}
}

func TestValidateRoutingRules(t *testing.T) {
	tt := []struct {
		name         string
		rules        []RuleDraft
		numGroups    int
		numProviders int
		err          error
	}{
		{
			name: "success.valid",
			rules: []RuleDraft{
				{Type: RuleDomainSuffix, Value: sp("x.com"), Target: RefDraft{Kind: PolicyDirect}},
				{Type: RuleRuleSet, ProviderIdx: ip(0), Target: RefDraft{Kind: PolicyDirect}},
				{Type: RuleMatch, Target: RefDraft{Kind: PolicyGroup, GroupIdx: ip(0)}},
			},
			numGroups:    1,
			numProviders: 1,
		},
		{
			name:  "error.bad_type",
			rules: []RuleDraft{{Type: "NOPE", Target: RefDraft{Kind: PolicyDirect}}},
			err:   ErrUnknownRuleType,
		},
		{
			name:  "error.no_value",
			rules: []RuleDraft{{Type: RuleDomain, Target: RefDraft{Kind: PolicyDirect}}},
			err:   ErrRuleValueRequired,
		},
		{
			name: "error.match_not_last",
			rules: []RuleDraft{
				{Type: RuleMatch, Target: RefDraft{Kind: PolicyDirect}},
				{Type: RuleDomain, Value: sp("x"), Target: RefDraft{Kind: PolicyDirect}},
			},
			err: ErrMatchNotLast,
		},
		{
			name:      "error.group_oob",
			rules:     []RuleDraft{{Type: RuleMatch, Target: RefDraft{Kind: PolicyGroup, GroupIdx: ip(3)}}},
			numGroups: 1,
			err:       ErrGroupRefRange,
		},
		{
			name:  "error.bad_ref",
			rules: []RuleDraft{{Type: RuleMatch, Target: RefDraft{Kind: PolicyInbound}}}, // inbound without id
			err:   ErrBadRef,
		},
		{
			name:         "error.ruleset_provider_oob",
			rules:        []RuleDraft{{Type: RuleRuleSet, ProviderIdx: ip(2), Target: RefDraft{Kind: PolicyDirect}}},
			numProviders: 1,
			err:          ErrProviderRefRange,
		},
		{
			name:  "error.ruleset_no_provider",
			rules: []RuleDraft{{Type: RuleRuleSet, Target: RefDraft{Kind: PolicyDirect}}}, // ProviderIdx nil
			err:   ErrProviderRefRange,
		},
		{
			name:  "error.value_on_match",
			rules: []RuleDraft{{Type: RuleMatch, Value: sp("x"), Target: RefDraft{Kind: PolicyDirect}}},
			err:   ErrRulePayloadNotAllowed,
		},
		{
			name:  "error.no_resolve_unsupported",
			rules: []RuleDraft{{Type: RuleDomain, Value: sp("x.com"), NoResolve: true, Target: RefDraft{Kind: PolicyDirect}}},
			err:   ErrNoResolveUnsupported,
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.ErrorIs(t, ValidateRoutingRules(tc.rules, tc.numGroups, tc.numProviders), tc.err)
		})
	}
}

func TestValidateProxyGroups(t *testing.T) {
	tt := []struct {
		name   string
		groups []GroupDraft
		err    error
	}{
		{
			name: "success.valid",
			groups: []GroupDraft{
				{Name: "smart", Type: GroupSelect, Members: []RefDraft{{Kind: PolicyDirect}}},
				{Name: "Conn", Type: GroupSelect, Members: []RefDraft{{Kind: PolicyGroup, GroupIdx: ip(0)}}},
			},
		},
		{
			name:   "error.no_name",
			groups: []GroupDraft{{Type: GroupSelect, Members: []RefDraft{{Kind: PolicyDirect}}}},
			err:    ErrGroupNameEmpty,
		},
		{
			name: "error.dup_name",
			groups: []GroupDraft{
				{Name: "g", Type: GroupSelect, Members: []RefDraft{{Kind: PolicyDirect}}},
				{Name: "g", Type: GroupSelect, Members: []RefDraft{{Kind: PolicyDirect}}},
			},
			err: ErrGroupNameTaken,
		},
		{
			name:   "error.bad_type",
			groups: []GroupDraft{{Name: "g", Type: "nope", Members: []RefDraft{{Kind: PolicyDirect}}}},
			err:    ErrGroupUnknownType,
		},
		{
			name:   "error.no_members",
			groups: []GroupDraft{{Name: "g", Type: GroupSelect}},
			err:    ErrGroupNoMembers,
		},
		{
			name: "error.cycle",
			groups: []GroupDraft{
				{Name: "a", Type: GroupSelect, Members: []RefDraft{{Kind: PolicyGroup, GroupIdx: ip(1)}}},
				{Name: "b", Type: GroupSelect, Members: []RefDraft{{Kind: PolicyGroup, GroupIdx: ip(0)}}},
			},
			err: ErrGroupCycle,
		},
		{
			name:   "error.field_on_select",
			groups: []GroupDraft{{Name: "g", Type: GroupSelect, Interval: ip(300), Members: []RefDraft{{Kind: PolicyDirect}}}},
			err:    ErrGroupFieldNotAllowed,
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.ErrorIs(t, ValidateProxyGroups(tc.groups), tc.err)
		})
	}
}

func TestValidateRuleProviders(t *testing.T) {
	tt := []struct {
		name  string
		provs []RuleProvider
		err   error
	}{
		{
			name:  "success.valid",
			provs: []RuleProvider{{Name: "geosite", Behavior: "domain", Format: "mrs", URL: "https://x/geosite.mrs"}},
		},
		{
			name:  "error.empty_name",
			provs: []RuleProvider{{Behavior: "domain", Format: "mrs", URL: "https://x"}},
			err:   ErrProviderNameEmpty,
		},
		{
			name:  "error.bad_behavior",
			provs: []RuleProvider{{Name: "p", Behavior: "nope", Format: "mrs", URL: "https://x"}},
			err:   ErrProviderBadBehavior,
		},
		{
			name:  "error.bad_format",
			provs: []RuleProvider{{Name: "p", Behavior: "domain", Format: "nope", URL: "https://x"}},
			err:   ErrProviderBadFormat,
		},
		{
			name:  "error.empty_url",
			provs: []RuleProvider{{Name: "p", Behavior: "domain", Format: "mrs"}},
			err:   ErrProviderURLEmpty,
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.ErrorIs(t, ValidateRuleProviders(tc.provs), tc.err)
		})
	}
}

func TestValidateProfile(t *testing.T) {
	tt := []struct {
		name    string
		profile Profile
		err     error
	}{
		{
			name:    "success.valid",
			profile: Profile{Title: "Freedom", Filename: "freedom.yaml", UpdateInterval: 1},
		},
		{
			name:    "error.title_empty",
			profile: Profile{Title: "", Filename: "freedom.yaml", UpdateInterval: 1},
			err:     ErrProfileTitleEmpty,
		},
		{
			name:    "error.filename_empty",
			profile: Profile{Title: "Freedom", Filename: "", UpdateInterval: 1},
			err:     ErrProfileFilenameEmpty,
		},
		{
			name:    "error.filename_path_separator",
			profile: Profile{Title: "Freedom", Filename: "sub/dir.yaml", UpdateInterval: 1},
			err:     ErrProfileFilenameInvalid,
		},
		{
			name:    "error.filename_control_char",
			profile: Profile{Title: "Freedom", Filename: "a\nb.yaml", UpdateInterval: 1},
			err:     ErrProfileFilenameInvalid,
		},
		{
			name:    "error.interval_zero",
			profile: Profile{Title: "Freedom", Filename: "freedom.yaml", UpdateInterval: 0},
			err:     ErrProfileUpdateIntervalInvalid,
		},
		{
			name:    "error.interval_negative",
			profile: Profile{Title: "Freedom", Filename: "freedom.yaml", UpdateInterval: -3},
			err:     ErrProfileUpdateIntervalInvalid,
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.ErrorIs(t, ValidateProfile(tc.profile), tc.err)
		})
	}
}
