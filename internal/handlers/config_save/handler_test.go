package config_save

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/postlog/subgen/internal/mihomo"
)

// wantSave is the SaveMihomoConfig argument set the "MATCH→direct" body parses into:
// one MATCH rule, no groups (empty, non-nil), no providers, the given base YAML.
func wantSave() (rules []mihomo.RoutingRule, groups []mihomo.ProxyGroup, rps []mihomo.RuleProvider) {
	return []mihomo.RoutingRule{{Type: mihomo.RuleMatch, Target: mihomo.PolicyRef{Kind: mihomo.PolicyDirect}}},
		[]mihomo.ProxyGroup{}, nil
}

type mocks struct {
	routing *MockmihomoSaver
}

func TestHandler_ServeHTTP(t *testing.T) {
	targetErr := errors.New("test")

	const matchOnly = `{"baseYAML":"dns: {}","rules":[{"type":"MATCH","target":{"kind":"direct"}}]}`

	tt := []struct {
		name       string
		body       string
		buildMocks func(m *mocks)
		wantOK     bool
	}{
		{
			name: "error.bad_json",
			body: `{bad`,
		},
		{
			name: "error.invalid_yaml",
			body: `{"baseYAML":"foo: [unclosed"}`,
		},
		{
			name: "error.generated_section",
			body: `{"baseYAML":"rules: []"}`,
		},
		{
			name: "error.match_not_last",
			body: `{"baseYAML":"dns: {}","rules":[` +
				`{"type":"MATCH","target":{"kind":"direct"}},` +
				`{"type":"DOMAIN","value":"example.com","target":{"kind":"direct"}}]}`,
		},
		{
			name: "error.save",
			body: matchOnly,
			buildMocks: func(m *mocks) {
				rules, groups, rps := wantSave()
				m.routing.EXPECT().SaveMihomoConfig(gomock.Any(), rules, groups, rps, "dns: {}").Return(targetErr)
			},
		},
		{
			name:   "success",
			body:   matchOnly,
			wantOK: true,
			buildMocks: func(m *mocks) {
				rules, groups, rps := wantSave()
				m.routing.EXPECT().SaveMihomoConfig(gomock.Any(), rules, groups, rps, "dns: {}").Return(nil)
			},
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			m := &mocks{routing: NewMockmihomoSaver(ctrl)}
			if tc.buildMocks != nil {
				tc.buildMocks(m)
			}

			h := New(m.routing)
			req := httptest.NewRequest(http.MethodPost, "/admin/api/config/save", strings.NewReader(tc.body))

			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			var body struct {
				OK bool `json:"ok"`
			}

			require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
			assert.Equal(t, tc.wantOK, body.OK)
		})
	}
}
