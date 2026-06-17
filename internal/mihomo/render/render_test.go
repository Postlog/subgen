package render

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/mihomo"
	"github.com/postlog/subgen/internal/utils"
)

// fixed UUIDs so the rendered proxies block is deterministic and can be asserted whole.
var (
	uuidNL2smart = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	uuidNL2force = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	uuidRU1force = uuid.MustParse("33333333-3333-3333-3333-333333333333")
)

// fullOptions: a "smart" wrapper (→DIRECT) and a "Connection" selector whose members
// are that group + two inbounds (10, 20). Rules: a GEOIP/no-resolve to DIRECT, a
// RULE-SET to inbound 30, a RULE-SET to inbound 999 (which the subscriber lacks → the
// rule is dropped), and MATCH→Connection. One mirrored rule-provider.
func fullOptions() Options {
	return Options{
		BaseYAML: "mode: rule\ndns:\n  enable: true\n  enhanced-mode: fake-ip\n",
		Groups: []mihomo.ProxyGroup{
			{ID: 1, Name: "smart", Type: mihomo.GroupSelect, Members: []mihomo.PolicyRef{{Kind: mihomo.PolicyDirect}}},
			{ID: 2, Name: "Connection", Type: mihomo.GroupSelect, Members: []mihomo.PolicyRef{
				{Kind: mihomo.PolicyGroup, GroupID: utils.Ptr[int64](1)},
				{Kind: mihomo.PolicyInbound, InboundID: utils.Ptr[int64](10)},
				{Kind: mihomo.PolicyInbound, InboundID: utils.Ptr[int64](20)},
			}},
		},
		Rules: []mihomo.RoutingRule{
			{Type: mihomo.RuleGeoIP, Value: utils.Ptr("private"), NoResolve: utils.Ptr(true), Target: &mihomo.PolicyRef{Kind: mihomo.PolicyDirect}},
			{Type: mihomo.RuleRuleSet, ProviderID: utils.Ptr[int64](7), Target: &mihomo.PolicyRef{Kind: mihomo.PolicyInbound, InboundID: utils.Ptr[int64](30)}},
			// inbound 999: the subscriber lacks it → the rule is dropped (target unresolved).
			{Type: mihomo.RuleRuleSet, ProviderID: utils.Ptr[int64](7), Target: &mihomo.PolicyRef{Kind: mihomo.PolicyInbound, InboundID: utils.Ptr[int64](999)}},
			{Type: mihomo.RuleMatch, Target: &mihomo.PolicyRef{Kind: mihomo.PolicyGroup, GroupID: utils.Ptr[int64](2)}},
		},
		Providers: []mihomo.RuleProvider{
			{ID: 7, Name: "allow", Behavior: "domain", Format: "mrs", Mirror: true, URL: "https://example/x.mrs", Interval: 86400},
		},
		PublicBase: "https://ru1.example:2097",
	}
}

func fullSub() *entity.Subscriber {
	return &entity.Subscriber{
		SubID: "abc",
		Proxies: []entity.Proxy{
			{
				Name: "smart-NL2", InboundID: 30,
				Server: "nl2.example", Port: 35740, UUID: uuidNL2smart, Network: "tcp",
				Security: "reality", PublicKey: "PBK", ShortID: "sid", ServerName: "www.sony.com", Fingerprint: "chrome",
			},
			{
				Name: "force-NL2", InboundID: 10,
				Server: "nl2.example", Port: 35741, UUID: uuidNL2force, Network: "tcp",
				Security: "reality", PublicKey: "PBK", ShortID: "sid", ServerName: "www.sony.com",
			},
			{
				Name: "force-RU1", InboundID: 20,
				Server: "ru1.example", Port: 8443, UUID: uuidRU1force, Network: "tcp",
				Security: "tls", Flow: "xtls-rprx-vision", ALPN: []string{"http/1.1"},
			},
		},
	}
}

// fullWant is the whole expected rendered doc for fullSub()/fullOptions(): base knobs
// preserved, the three proxies mapped (reality with fingerprint, reality without, tls
// with alpn+flow), groups resolved (smart→DIRECT fallback; Connection→group+two
// inbound proxy names), rules retargeted to inbound proxy names with the missing-inbound
// rule dropped and MATCH last, and the mirrored rule-provider pointing at /rules/.
const fullWant = `
mode: rule
dns:
  enable: true
  enhanced-mode: fake-ip
proxies:
  - name: smart-NL2
    type: vless
    server: nl2.example
    port: 35740
    uuid: 11111111-1111-1111-1111-111111111111
    udp: true
    network: tcp
    tls: true
    servername: www.sony.com
    client-fingerprint: chrome
    reality-opts:
      public-key: PBK
      short-id: sid
  - name: force-NL2
    type: vless
    server: nl2.example
    port: 35741
    uuid: 22222222-2222-2222-2222-222222222222
    udp: true
    network: tcp
    tls: true
    servername: www.sony.com
    reality-opts:
      public-key: PBK
      short-id: sid
  - name: force-RU1
    type: vless
    server: ru1.example
    port: 8443
    uuid: 33333333-3333-3333-3333-333333333333
    udp: true
    network: tcp
    flow: xtls-rprx-vision
    tls: true
    servername: ru1.example
    alpn:
      - http/1.1
proxy-groups:
  - name: smart
    type: select
    proxies:
      - DIRECT
  - name: Connection
    type: select
    proxies:
      - smart
      - force-NL2
      - force-RU1
rules:
  - GEOIP,private,DIRECT,no-resolve
  - RULE-SET,allow,smart-NL2
  - MATCH,Connection
rule-providers:
  allow:
    type: http
    behavior: domain
    url: https://ru1.example:2097/rules/allow.mrs
    path: ./ruleset/allow.mrs
    format: mrs
    interval: 86400
`

// partialOptions: a "smart" wrapper (→DIRECT) and a "Conn" selector referencing that
// group + two inbounds (10, 20). Rules: a rule to inbound 30, a rule to inbound 20, and
// MATCH→Conn. No rule-providers.
func partialOptions() Options {
	return Options{
		BaseYAML: "mode: rule",
		Groups: []mihomo.ProxyGroup{
			{ID: 1, Name: "smart", Type: mihomo.GroupSelect, Members: []mihomo.PolicyRef{{Kind: mihomo.PolicyDirect}}},
			{ID: 2, Name: "Conn", Type: mihomo.GroupSelect, Members: []mihomo.PolicyRef{
				{Kind: mihomo.PolicyGroup, GroupID: utils.Ptr[int64](1)},
				{Kind: mihomo.PolicyInbound, InboundID: utils.Ptr[int64](10)},
				{Kind: mihomo.PolicyInbound, InboundID: utils.Ptr[int64](20)},
			}},
		},
		Rules: []mihomo.RoutingRule{
			{Type: mihomo.RuleDomainSuffix, Value: utils.Ptr("skip.example"), Target: &mihomo.PolicyRef{Kind: mihomo.PolicyInbound, InboundID: utils.Ptr[int64](30)}},
			{Type: mihomo.RuleDomain, Value: utils.Ptr("x.com"), Target: &mihomo.PolicyRef{Kind: mihomo.PolicyInbound, InboundID: utils.Ptr[int64](20)}},
			{Type: mihomo.RuleMatch, Target: &mihomo.PolicyRef{Kind: mihomo.PolicyGroup, GroupID: utils.Ptr[int64](2)}},
		},
	}
}

func TestRender(t *testing.T) {
	tt := []struct {
		name string

		sub  *entity.Subscriber
		opts Options

		// wantYAML is the whole expected rendered doc (compared structurally, decoded);
		// empty when wantErr is set.
		wantYAML string
		wantErr  bool
	}{
		{
			name:     "success.groups_rules_proxies",
			sub:      fullSub(),
			opts:     fullOptions(),
			wantYAML: fullWant,
		},
		{
			// inbound 10 present; not 20 or 30. inbound-20 dropped from the selector,
			// inbound-30 + inbound-20 rules dropped, smart stays its DIRECT fallback.
			name: "success.missing_one_inbound",
			sub: &entity.Subscriber{SubID: "x", Proxies: []entity.Proxy{
				{Name: "force-NL2", InboundID: 10, Server: "nl2", Port: 1, UUID: uuidNL2force, Network: "tcp", Security: "tls"},
			}},
			opts: partialOptions(),
			wantYAML: `
mode: rule
proxies:
  - name: force-NL2
    type: vless
    server: nl2
    port: 1
    uuid: 22222222-2222-2222-2222-222222222222
    udp: true
    network: tcp
    tls: true
    servername: nl2
proxy-groups:
  - name: smart
    type: select
    proxies:
      - DIRECT
  - name: Conn
    type: select
    proxies:
      - smart
      - force-NL2
rules:
  - MATCH,Conn
`,
		},
		{
			// no proxies at all: both inbounds dropped (only the group ref remains in
			// Conn), every inbound rule dropped, smart falls back to DIRECT.
			name: "empty_subscriber",
			sub:  &entity.Subscriber{SubID: "x"},
			opts: partialOptions(),
			wantYAML: `
mode: rule
proxies: []
proxy-groups:
  - name: smart
    type: select
    proxies:
      - DIRECT
  - name: Conn
    type: select
    proxies:
      - smart
rules:
  - MATCH,Conn
`,
		},
		{
			// A RULE-SET whose provider id has no match (config-global providers make this
			// unreachable in practice) is dropped, like an unresolvable target.
			name: "ruleset_unknown_provider_dropped",
			sub:  &entity.Subscriber{SubID: "x"},
			opts: Options{
				BaseYAML: "mode: rule",
				Rules: []mihomo.RoutingRule{
					{Type: mihomo.RuleRuleSet, ProviderID: utils.Ptr[int64](99), Target: &mihomo.PolicyRef{Kind: mihomo.PolicyDirect}},
					{Type: mihomo.RuleMatch, Target: &mihomo.PolicyRef{Kind: mihomo.PolicyDirect}},
				},
			},
			wantYAML: `
mode: rule
proxies: []
proxy-groups: []
rules:
  - MATCH,DIRECT
`,
		},
		{
			// Logical rules (AND/OR/NOT) render the nested-condition syntax verbatim,
			// including a RULE-SET sub-condition (provider name from id) and a logical
			// condition nested inside another. This is the QUIC-block use case + nesting.
			name: "logical_rules",
			sub:  &entity.Subscriber{SubID: "x"},
			opts: Options{
				BaseYAML: "mode: rule",
				Providers: []mihomo.RuleProvider{
					{ID: 5, Name: "ads", Behavior: "domain", Format: "mrs", URL: "https://up/ads.mrs", Interval: 3600},
				},
				Rules: []mihomo.RoutingRule{
					{Type: mihomo.RuleAnd, Target: &mihomo.PolicyRef{Kind: mihomo.PolicyRejectDrop}, Children: []mihomo.RoutingRule{
						{Type: mihomo.RuleNetwork, Value: utils.Ptr("UDP")},
						{Type: mihomo.RuleDstPort, Value: utils.Ptr("443")},
					}},
					{Type: mihomo.RuleOr, Target: &mihomo.PolicyRef{Kind: mihomo.PolicyDirect}, Children: []mihomo.RoutingRule{
						{Type: mihomo.RuleAnd, Children: []mihomo.RoutingRule{
							{Type: mihomo.RuleDomainKeyword, Value: utils.Ptr("ad")},
							{Type: mihomo.RuleNetwork, Value: utils.Ptr("tcp")},
						}},
						{Type: mihomo.RuleRuleSet, ProviderID: utils.Ptr[int64](5)},
					}},
					{Type: mihomo.RuleNot, Target: &mihomo.PolicyRef{Kind: mihomo.PolicyReject}, Children: []mihomo.RoutingRule{
						{Type: mihomo.RuleDomainSuffix, Value: utils.Ptr("ok.com")},
					}},
					{Type: mihomo.RuleMatch, Target: &mihomo.PolicyRef{Kind: mihomo.PolicyDirect}},
				},
			},
			wantYAML: `
mode: rule
proxies: []
proxy-groups: []
rules:
  - AND,((NETWORK,UDP),(DST-PORT,443)),REJECT-DROP
  - OR,((AND,((DOMAIN-KEYWORD,ad),(NETWORK,tcp))),(RULE-SET,ads)),DIRECT
  - NOT,((DOMAIN-SUFFIX,ok.com)),REJECT
  - MATCH,DIRECT
rule-providers:
  ads:
    type: http
    behavior: domain
    url: https://up/ads.mrs
    path: ./ruleset/ads.mrs
    format: mrs
    interval: 3600
`,
		},
		{
			name:    "error.invalid_base_yaml",
			sub:     &entity.Subscriber{SubID: "x"},
			opts:    Options{BaseYAML: "mode: rule\n\tbad: : indent"},
			wantErr: true,
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			out, err := Render(tc.sub, tc.opts)

			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			var got, want map[string]any
			require.NoError(t, yaml.Unmarshal(out, &got))
			require.NoError(t, yaml.Unmarshal([]byte(tc.wantYAML), &want))

			assert.Equal(t, want, got)
		})
	}
}
