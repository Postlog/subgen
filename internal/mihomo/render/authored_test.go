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

func TestRenderAuthoredProvider(t *testing.T) {
	tt := []struct {
		name     string
		matchers []mihomo.RoutingRule
		want     string
	}{
		{name: "empty", want: ""},
		{
			name: "leaf_and_logical",
			matchers: []mihomo.RoutingRule{
				{Type: mihomo.RuleDomainKeyword, Value: utils.Ptr("ads")},
				{Type: mihomo.RuleAnd, Children: []mihomo.RoutingRule{
					{Type: mihomo.RuleNetwork, Value: utils.Ptr("udp")},
					{Type: mihomo.RuleOr, Children: []mihomo.RoutingRule{
						{Type: mihomo.RuleDstPort, Value: utils.Ptr("53")},
						{Type: mihomo.RuleDomainSuffix, Value: utils.Ptr("x.com")},
					}},
				}},
			},
			// one matcher per line, no target; logical nests with per-child parentheses.
			want: "DOMAIN-KEYWORD,ads\nAND,((NETWORK,udp),(OR,((DST-PORT,53),(DOMAIN-SUFFIX,x.com))))\n",
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, string(RenderAuthoredProvider(tc.matchers)))
		})
	}
}

func TestRenderProxiesPayload(t *testing.T) {
	t.Parallel()

	sub := &entity.Subscriber{Proxies: []entity.Proxy{
		{Name: "RU1", Server: "ru1", Port: 8443, UUID: uuid.MustParse("33333333-3333-3333-3333-333333333333"), Network: "tcp", Security: "tls"},
	}}

	out, err := RenderProxiesPayload(sub)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, yaml.Unmarshal(out, &got))

	// A proxy-provider payload is a document with a top-level proxies: array.
	want := map[string]any{"proxies": []any{map[string]any{
		"name": "RU1", "type": "vless", "server": "ru1", "port": 8443,
		"uuid": "33333333-3333-3333-3333-333333333333", "udp": true, "network": "tcp",
		"tls": true, "servername": "ru1",
	}}}
	assert.Equal(t, want, got)
}
