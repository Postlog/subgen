package mihomo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/utils"
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
						{Kind: PolicyGroup, GroupIdx: utils.Ptr(0)},
						{Kind: PolicyInbound, InboundID: utils.Ptr[int64](5)},
					}},
				},
				Rules: []RuleDraft{
					{Type: RuleDomainSuffix, Value: utils.Ptr("example.com"), Target: &RefDraft{Kind: PolicyInbound, InboundID: utils.Ptr[int64](5)}},
					{Type: RuleRuleSet, ProviderIdx: utils.Ptr(0), Target: &RefDraft{Kind: PolicyDirect}},
					{Type: RuleMatch, Target: &RefDraft{Kind: PolicyGroup, GroupIdx: utils.Ptr(1)}},
				},
				Providers: []RuleProvider{{Name: "allow", Source: RuleProviderExternal, Behavior: "domain", Format: "mrs", URL: "https://x"}},
			},
		},
		{
			// A logical rule (AND/OR/NOT) decodes its recursive sub-conditions: a RULE-SET
			// condition carries a providerIdx, a nested logical condition its own children.
			// The logical rule itself carries no value/provider, just the conditions + target.
			name: "success.logical_rule_conditions",
			body: `{
				"providers": [{"name":"ads","behavior":"domain","format":"mrs","url":"https://x"}],
				"rules": [
					{"type":"AND","target":{"kind":"reject-drop"},"children":[
						{"type":"NETWORK","value":"UDP"},
						{"type":"OR","children":[
							{"type":"DST-PORT","value":"443"},
							{"type":"RULE-SET","providerIdx":0}
						]}
					]}
				]
			}`,
			want: ConfigDraft{
				Groups: []GroupDraft{},
				Rules: []RuleDraft{
					{Type: RuleAnd, Target: &RefDraft{Kind: PolicyRejectDrop}, Children: []RuleDraft{
						{Type: RuleNetwork, Value: utils.Ptr("UDP")},
						{Type: RuleOr, Children: []RuleDraft{
							{Type: RuleDstPort, Value: utils.Ptr("443")},
							{Type: RuleRuleSet, ProviderIdx: utils.Ptr(0)},
						}},
					}},
				},
				Providers: []RuleProvider{{Name: "ads", Source: RuleProviderExternal, Behavior: "domain", Format: "mrs", URL: "https://x"}},
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
				Providers: []RuleProvider{{Name: "", Source: RuleProviderExternal, Behavior: "domain", Format: "mrs", URL: "https://x"}},
			},
		},
		{
			// An authored provider: source=authored carries a matcher tree (decoded into
			// RoutingRule with no target), and subgen normalizes behavior/format to
			// classical/text and clears url/mirror. proxiesInterval rides on the profile.
			name: "success.authored_provider",
			body: `{
				"proxiesInterval": 1800,
				"providers": [
					{"name":"reject-set","source":"authored","behavior":"x","format":"y","url":"https://drop","matchers":[
						{"type":"DOMAIN-KEYWORD","value":"ads"},
						{"type":"AND","children":[{"type":"NETWORK","value":"udp"},{"type":"DST-PORT","value":"53"}]}
					]}
				]
			}`,
			want: ConfigDraft{
				Groups:  []GroupDraft{},
				Rules:   []RuleDraft{},
				Profile: Profile{ProxiesInterval: 1800},
				Providers: []RuleProvider{{
					Name: "reject-set", Source: RuleProviderAuthored, Behavior: "classical", Format: "text",
					Matchers: []RoutingRule{
						{Type: RuleDomainKeyword, Value: utils.Ptr("ads")},
						{Type: RuleAnd, Children: []RoutingRule{
							{Type: RuleNetwork, Value: utils.Ptr("udp")},
							{Type: RuleDstPort, Value: utils.Ptr("53")},
						}},
					},
				}},
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
