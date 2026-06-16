package mihomo

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/utils"
)

func TestValidateBaseYAML(t *testing.T) {
	tt := []struct {
		name string
		base string
		err  error
	}{
		{name: "success.valid", base: "mode: rule\ndns: {}"},
		{name: "error.invalid_yaml", base: "foo: [unclosed", err: ErrBaseYAMLInvalid},
		{name: "error.generated_section", base: "rules: []", err: ErrGeneratedKeyPresent},
		{name: "error.sub_rules_reserved", base: "sub-rules: {}", err: ErrGeneratedKeyPresent},
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
				{Type: RuleDomainSuffix, Value: utils.Ptr("x.com"), Target: &RefDraft{Kind: PolicyDirect}},
				{Type: RuleRuleSet, ProviderIdx: utils.Ptr(0), Target: &RefDraft{Kind: PolicyDirect}},
				{Type: RuleMatch, Target: &RefDraft{Kind: PolicyGroup, GroupIdx: utils.Ptr(0)}},
			},
			numGroups:    1,
			numProviders: 1,
		},
		{
			name:  "error.bad_type",
			rules: []RuleDraft{{Type: "NOPE", Target: &RefDraft{Kind: PolicyDirect}}},
			err:   ErrUnknownRuleType,
		},
		{
			name:  "error.no_value",
			rules: []RuleDraft{{Type: RuleDomain, Target: &RefDraft{Kind: PolicyDirect}}},
			err:   ErrRuleValueRequired,
		},
		{
			name: "error.match_not_last",
			rules: []RuleDraft{
				{Type: RuleMatch, Target: &RefDraft{Kind: PolicyDirect}},
				{Type: RuleDomain, Value: utils.Ptr("x"), Target: &RefDraft{Kind: PolicyDirect}},
			},
			err: ErrMatchNotLast,
		},
		{
			name:      "error.group_oob",
			rules:     []RuleDraft{{Type: RuleMatch, Target: &RefDraft{Kind: PolicyGroup, GroupIdx: utils.Ptr(3)}}},
			numGroups: 1,
			err:       ErrGroupRefRange,
		},
		{
			name:  "error.bad_ref",
			rules: []RuleDraft{{Type: RuleMatch, Target: &RefDraft{Kind: PolicyInbound}}}, // inbound without id
			err:   ErrBadRef,
		},
		{
			name:         "error.ruleset_provider_oob",
			rules:        []RuleDraft{{Type: RuleRuleSet, ProviderIdx: utils.Ptr(2), Target: &RefDraft{Kind: PolicyDirect}}},
			numProviders: 1,
			err:          ErrProviderRefRange,
		},
		{
			name:  "error.ruleset_no_provider",
			rules: []RuleDraft{{Type: RuleRuleSet, Target: &RefDraft{Kind: PolicyDirect}}}, // ProviderIdx nil
			err:   ErrProviderRefRange,
		},
		{
			name:  "error.value_on_match",
			rules: []RuleDraft{{Type: RuleMatch, Value: utils.Ptr("x"), Target: &RefDraft{Kind: PolicyDirect}}},
			err:   ErrRulePayloadNotAllowed,
		},
		{
			name:  "error.no_resolve_unsupported",
			rules: []RuleDraft{{Type: RuleDomain, Value: utils.Ptr("x.com"), NoResolve: utils.Ptr(true), Target: &RefDraft{Kind: PolicyDirect}}},
			err:   ErrNoResolveUnsupported,
		},
		{
			name: "success.logical_and",
			rules: []RuleDraft{
				{Type: RuleAnd, Target: &RefDraft{Kind: PolicyRejectDrop}, Children: []RuleDraft{
					{Type: RuleNetwork, Value: utils.Ptr("UDP")},
					{Type: RuleDstPort, Value: utils.Ptr("443")},
				}},
			},
		},
		{
			name: "success.logical_nested_with_ruleset_condition",
			rules: []RuleDraft{
				{Type: RuleOr, Target: &RefDraft{Kind: PolicyDirect}, Children: []RuleDraft{
					{Type: RuleNot, Children: []RuleDraft{{Type: RuleDomainSuffix, Value: utils.Ptr("ok.com")}}},
					{Type: RuleRuleSet, ProviderIdx: utils.Ptr(0)},
				}},
			},
			numProviders: 1,
		},
		{
			name: "error.and_too_few_conditions",
			rules: []RuleDraft{
				{Type: RuleAnd, Target: &RefDraft{Kind: PolicyDirect}, Children: []RuleDraft{{Type: RuleNetwork, Value: utils.Ptr("UDP")}}},
			},
			err: ErrLogicalArity,
		},
		{
			name: "error.not_arity",
			rules: []RuleDraft{
				{Type: RuleNot, Target: &RefDraft{Kind: PolicyDirect}, Children: []RuleDraft{
					{Type: RuleNetwork, Value: utils.Ptr("UDP")},
					{Type: RuleDstPort, Value: utils.Ptr("443")},
				}},
			},
			err: ErrNotArity,
		},
		{
			name: "error.match_in_condition",
			rules: []RuleDraft{
				{Type: RuleAnd, Target: &RefDraft{Kind: PolicyDirect}, Children: []RuleDraft{
					{Type: RuleMatch},
					{Type: RuleNetwork, Value: utils.Ptr("UDP")},
				}},
			},
			err: ErrMatchChild,
		},
		{
			name: "error.conditions_on_simple",
			rules: []RuleDraft{
				{Type: RuleDomain, Value: utils.Ptr("x.com"), Target: &RefDraft{Kind: PolicyDirect}, Children: []RuleDraft{
					{Type: RuleNetwork, Value: utils.Ptr("UDP")},
				}},
			},
			err: ErrChildrenNotAllowed,
		},
		{
			name: "error.logical_with_value",
			rules: []RuleDraft{
				{Type: RuleAnd, Value: utils.Ptr("x"), Target: &RefDraft{Kind: PolicyDirect}, Children: []RuleDraft{
					{Type: RuleNetwork, Value: utils.Ptr("UDP")},
					{Type: RuleDstPort, Value: utils.Ptr("443")},
				}},
			},
			err: ErrRulePayloadNotAllowed,
		},
		{
			name: "error.condition_provider_oob",
			rules: []RuleDraft{
				{Type: RuleAnd, Target: &RefDraft{Kind: PolicyDirect}, Children: []RuleDraft{
					{Type: RuleRuleSet, ProviderIdx: utils.Ptr(5)},
					{Type: RuleNetwork, Value: utils.Ptr("UDP")},
				}},
			},
			numProviders: 1,
			err:          ErrProviderRefRange,
		},
		{
			name: "error.unknown_condition_type",
			rules: []RuleDraft{
				{Type: RuleAnd, Target: &RefDraft{Kind: PolicyDirect}, Children: []RuleDraft{
					{Type: "NOPE"},
					{Type: RuleNetwork, Value: utils.Ptr("UDP")},
				}},
			},
			err: ErrUnknownRuleType,
		},
		{
			name: "error.condition_value_required",
			rules: []RuleDraft{
				{Type: RuleAnd, Target: &RefDraft{Kind: PolicyDirect}, Children: []RuleDraft{
					{Type: RuleDomain},
					{Type: RuleNetwork, Value: utils.Ptr("UDP")},
				}},
			},
			err: ErrRuleValueRequired,
		},
		{
			// A client may send noResolve:false unconditionally (no omitempty); an explicit
			// false on a logical rule is harmless and must NOT be rejected.
			name: "success.logical_explicit_false_no_resolve",
			rules: []RuleDraft{
				{Type: RuleAnd, NoResolve: utils.Ptr(false), Target: &RefDraft{Kind: PolicyDirect}, Children: []RuleDraft{
					{Type: RuleNetwork, Value: utils.Ptr("UDP")},
					{Type: RuleDstPort, Value: utils.Ptr("443")},
				}},
			},
		},
		{
			// A meaningful (true) no-resolve on a sub-rule is rejected — sub-rules carry no params.
			name: "error.child_true_no_resolve",
			rules: []RuleDraft{
				{Type: RuleAnd, Target: &RefDraft{Kind: PolicyDirect}, Children: []RuleDraft{
					{Type: RuleIPCIDR, Value: utils.Ptr("1.1.1.1/32"), NoResolve: utils.Ptr(true)},
					{Type: RuleDstPort, Value: utils.Ptr("443")},
				}},
			},
			err: ErrNoResolveUnsupported,
		},
		{
			// A top-level rule must carry a target (a sub-rule's nil-target shape is invalid here).
			name:  "error.target_required",
			rules: []RuleDraft{{Type: RuleDomain, Value: utils.Ptr("x.com")}},
			err:   ErrTargetRequired,
		},
		{
			// A sub-rule must NOT carry a target — it is a matcher, not a routing decision.
			name: "error.child_has_target",
			rules: []RuleDraft{
				{Type: RuleAnd, Target: &RefDraft{Kind: PolicyDirect}, Children: []RuleDraft{
					{Type: RuleNetwork, Value: utils.Ptr("UDP"), Target: &RefDraft{Kind: PolicyDirect}},
					{Type: RuleDstPort, Value: utils.Ptr("443")},
				}},
			},
			err: ErrChildTarget,
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
				{Name: "Conn", Type: GroupSelect, Members: []RefDraft{{Kind: PolicyGroup, GroupIdx: utils.Ptr(0)}}},
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
				{Name: "a", Type: GroupSelect, Members: []RefDraft{{Kind: PolicyGroup, GroupIdx: utils.Ptr(1)}}},
				{Name: "b", Type: GroupSelect, Members: []RefDraft{{Kind: PolicyGroup, GroupIdx: utils.Ptr(0)}}},
			},
			err: ErrGroupCycle,
		},
		{
			name:   "error.field_on_select",
			groups: []GroupDraft{{Name: "g", Type: GroupSelect, Interval: utils.Ptr(300), Members: []RefDraft{{Kind: PolicyDirect}}}},
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
