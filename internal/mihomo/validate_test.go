package mihomo

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func i64(v int64) *int64 { return &v }

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
		name      string
		rules     []RoutingRule
		numGroups int
		err       error
	}{
		{
			name: "success.valid",
			rules: []RoutingRule{
				{Type: RuleDomainSuffix, Value: "x.com", Target: PolicyRef{Kind: PolicyDirect}},
				{Type: RuleMatch, Target: PolicyRef{Kind: PolicyGroup, GroupID: i64(0)}},
			},
			numGroups: 1,
		},
		{
			name:  "error.bad_type",
			rules: []RoutingRule{{Type: "NOPE", Target: PolicyRef{Kind: PolicyDirect}}},
			err:   ErrUnknownRuleType,
		},
		{
			name:  "error.no_value",
			rules: []RoutingRule{{Type: RuleDomain, Target: PolicyRef{Kind: PolicyDirect}}},
			err:   ErrRuleValueRequired,
		},
		{
			name: "error.match_not_last",
			rules: []RoutingRule{
				{Type: RuleMatch, Target: PolicyRef{Kind: PolicyDirect}},
				{Type: RuleDomain, Value: "x", Target: PolicyRef{Kind: PolicyDirect}},
			},
			err: ErrMatchNotLast,
		},
		{
			name:      "error.group_oob",
			rules:     []RoutingRule{{Type: RuleMatch, Target: PolicyRef{Kind: PolicyGroup, GroupID: i64(3)}}},
			numGroups: 1,
			err:       ErrGroupRefRange,
		},
		{
			name:  "error.bad_ref",
			rules: []RoutingRule{{Type: RuleMatch, Target: PolicyRef{Kind: PolicyInbound}}}, // inbound without id
			err:   ErrBadRef,
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.ErrorIs(t, ValidateRoutingRules(tc.rules, tc.numGroups), tc.err)
		})
	}
}

func TestValidateProxyGroups(t *testing.T) {
	tt := []struct {
		name   string
		groups []ProxyGroup
		err    error
	}{
		{
			name: "success.valid",
			groups: []ProxyGroup{
				{Name: "smart", Type: GroupSelect, Members: []PolicyRef{{Kind: PolicyDirect}}},
				{Name: "Conn", Type: GroupSelect, Members: []PolicyRef{{Kind: PolicyGroup, GroupID: i64(0)}}},
			},
		},
		{
			name:   "error.no_name",
			groups: []ProxyGroup{{Type: GroupSelect, Members: []PolicyRef{{Kind: PolicyDirect}}}},
			err:    ErrGroupNameEmpty,
		},
		{
			name: "error.dup_name",
			groups: []ProxyGroup{
				{Name: "g", Type: GroupSelect, Members: []PolicyRef{{Kind: PolicyDirect}}},
				{Name: "g", Type: GroupSelect, Members: []PolicyRef{{Kind: PolicyDirect}}},
			},
			err: ErrGroupNameTaken,
		},
		{
			name:   "error.bad_type",
			groups: []ProxyGroup{{Name: "g", Type: "nope", Members: []PolicyRef{{Kind: PolicyDirect}}}},
			err:    ErrGroupUnknownType,
		},
		{
			name:   "error.no_members",
			groups: []ProxyGroup{{Name: "g", Type: GroupSelect}},
			err:    ErrGroupNoMembers,
		},
		{
			name: "error.cycle",
			groups: []ProxyGroup{
				{Name: "a", Type: GroupSelect, Members: []PolicyRef{{Kind: PolicyGroup, GroupID: i64(1)}}},
				{Name: "b", Type: GroupSelect, Members: []PolicyRef{{Kind: PolicyGroup, GroupID: i64(0)}}},
			},
			err: ErrGroupCycle,
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

func TestValidateRuleProviderRefs(t *testing.T) {
	provs := []RuleProvider{{Name: "geosite", Behavior: "domain", Format: "mrs", URL: "https://x"}}

	tt := []struct {
		name  string
		rules []RoutingRule
		err   error
	}{
		{
			name: "success.known_provider",
			rules: []RoutingRule{
				{Type: RuleRuleSet, Value: "geosite", Target: PolicyRef{Kind: PolicyDirect}},
				{Type: RuleDomainSuffix, Value: "x.com", Target: PolicyRef{Kind: PolicyDirect}},
			},
		},
		{
			name:  "error.unknown_provider",
			rules: []RoutingRule{{Type: RuleRuleSet, Value: "missing", Target: PolicyRef{Kind: PolicyDirect}}},
			err:   ErrRuleSetUnknownProvider,
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.ErrorIs(t, ValidateRuleProviderRefs(tc.rules, provs), tc.err)
		})
	}
}
