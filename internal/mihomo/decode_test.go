package mihomo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeConfig(t *testing.T) {
	tt := []struct {
		name string
		body string

		wantRules  []RoutingRule
		wantGroups []ProxyGroup
		wantProvs  []RuleProvider
		wantBase   string
	}{
		{
			// two groups: "smart" (→direct) and "Conn" (member group→idx 0, inbound 5);
			// rules: DOMAIN-SUFFIX→inbound 5, MATCH→group idx 1. Group refs decode to the
			// array INDEX (the SaveMihomoConfig convention), not a persisted id.
			name: "success.full",
			body: `{
				"baseYAML": "mode: rule",
				"groups": [
					{"name":"smart","type":"select","members":[{"kind":"direct"}]},
					{"name":"Conn","type":"select","members":[{"kind":"group","groupIdx":0},{"kind":"inbound","inboundId":5}]}
				],
				"rules": [
					{"type":"DOMAIN-SUFFIX","value":"example.com","target":{"kind":"inbound","inboundId":5}},
					{"type":"MATCH","target":{"kind":"group","groupIdx":1}}
				]
			}`,
			wantGroups: []ProxyGroup{
				{Name: "smart", Type: GroupSelect, Members: []PolicyRef{{Kind: PolicyDirect}}},
				{Name: "Conn", Type: GroupSelect, Members: []PolicyRef{
					{Kind: PolicyGroup, GroupID: i64(0)},
					{Kind: PolicyInbound, InboundID: i64(5)},
				}},
			},
			wantRules: []RoutingRule{
				{Type: RuleDomainSuffix, Value: "example.com", Target: PolicyRef{Kind: PolicyInbound, InboundID: i64(5)}},
				{Type: RuleMatch, Target: PolicyRef{Kind: PolicyGroup, GroupID: i64(1)}},
			},
			wantProvs: nil,
			wantBase:  "mode: rule",
		},
		{
			// Regression: a provider with an all-whitespace name was once silently dropped
			// (decode skip + frontend filter), so a half-filled row "saved" as a no-op.
			// Decode now KEEPS it (name trimmed to "") so ValidateRuleProviders can reject
			// the save (see TestValidateRuleProviders/error.empty_name → ErrProviderNameEmpty).
			name:       "success.nameless_provider_kept",
			body:       `{"providers":[{"name":"  ","behavior":"domain","format":"mrs","url":"https://x"}]}`,
			wantRules:  []RoutingRule{},
			wantGroups: []ProxyGroup{},
			wantProvs:  []RuleProvider{{Name: "", Behavior: "domain", Format: "mrs", URL: "https://x"}},
			wantBase:   "",
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rules, groups, provs, base, err := DecodeConfig([]byte(tc.body))

			require.NoError(t, err)
			assert.Equal(t, tc.wantRules, rules)
			assert.Equal(t, tc.wantGroups, groups)
			assert.Equal(t, tc.wantProvs, provs)
			assert.Equal(t, tc.wantBase, base)
		})
	}
}
