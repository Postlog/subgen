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
		want ConfigDraft
	}{
		{
			// two groups: "smart" (→direct) and "Conn" (member group→idx 0, inbound 5);
			// rules: DOMAIN-SUFFIX→inbound 5, RULE-SET→provider idx 0, MATCH→group idx 1.
			// Group/provider refs decode to the array INDEX (the SaveMihomoConfig
			// convention), not a persisted id. The profile title is padded to exercise trim.
			name: "success.full",
			body: `{
				"baseYAML": "mode: rule",
				"profileTitle": "  My VPN  ",
				"filename": "my.yaml",
				"profileUpdateInterval": 6,
				"groups": [
					{"name":"smart","type":"select","members":[{"kind":"direct"}]},
					{"name":"Conn","type":"select","members":[{"kind":"group","groupIdx":0},{"kind":"inbound","inboundId":5}]}
				],
				"providers": [
					{"name":"allow","behavior":"domain","format":"mrs","url":"https://x"}
				],
				"rules": [
					{"type":"DOMAIN-SUFFIX","value":"example.com","target":{"kind":"inbound","inboundId":5}},
					{"type":"RULE-SET","providerIdx":0,"target":{"kind":"direct"}},
					{"type":"MATCH","target":{"kind":"group","groupIdx":1}}
				]
			}`,
			want: ConfigDraft{
				BaseYAML: "mode: rule",
				Profile:  Profile{Title: "My VPN", Filename: "my.yaml", UpdateInterval: 6},
				Groups: []GroupDraft{
					{Name: "smart", Type: GroupSelect, Members: []RefDraft{{Kind: PolicyDirect}}},
					{Name: "Conn", Type: GroupSelect, Members: []RefDraft{
						{Kind: PolicyGroup, GroupIdx: ip(0)},
						{Kind: PolicyInbound, InboundID: i64(5)},
					}},
				},
				Rules: []RuleDraft{
					{Type: RuleDomainSuffix, Value: sp("example.com"), Target: RefDraft{Kind: PolicyInbound, InboundID: i64(5)}},
					{Type: RuleRuleSet, ProviderIdx: ip(0), Target: RefDraft{Kind: PolicyDirect}},
					{Type: RuleMatch, Target: RefDraft{Kind: PolicyGroup, GroupIdx: ip(1)}},
				},
				Providers: []RuleProvider{{Name: "allow", Behavior: "domain", Format: "mrs", URL: "https://x"}},
			},
		},
		{
			// Regression: a provider with an all-whitespace name was once silently dropped
			// (decode skip + frontend filter), so a half-filled row "saved" as a no-op.
			// Decode now KEEPS it (name trimmed to "") so ValidateRuleProviders can reject
			// the save (see TestValidateRuleProviders/error.empty_name → ErrProviderNameEmpty).
			name: "success.nameless_provider_kept",
			body: `{"providers":[{"name":"  ","behavior":"domain","format":"mrs","url":"https://x"}]}`,
			want: ConfigDraft{
				Groups:    []GroupDraft{},
				Rules:     []RuleDraft{},
				Providers: []RuleProvider{{Name: "", Behavior: "domain", Format: "mrs", URL: "https://x"}},
			},
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := DecodeConfig([]byte(tc.body))

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
