package config_get

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/mihomo"
	"github.com/postlog/subgen/internal/oas"
	"github.com/postlog/subgen/internal/utils"
)

func TestHandler_ConfigGet(t *testing.T) {
	internalErr := errors.New("db down")

	tt := []struct {
		name   string
		params oas.ConfigGetParams

		buildConfigsMock func(m *MockconfigResolver)
		buildRoutingMock func(m *MockmihomoReader)

		result oas.ConfigGetRes
		err    error
	}{
		{
			name:   "success.base_empty",
			params: oas.ConfigGetParams{},
			buildConfigsMock: func(m *MockconfigResolver) {
				m.EXPECT().BaseConfigID(gomock.Any(), entity.ConfigKindMihomo).Return(int64(0), false, nil)
			},
			// No config row yet → empty config means empty: profile knobs come back
			// zero-valued, no default substitution here.
			result: &oas.MihomoConfig{
				Groups:    []oas.MihomoGroup{},
				Rules:     []oas.MihomoRule{},
				Providers: []oas.MihomoProvider{},
			},
		},
		{
			name:   "success.base_populated",
			params: oas.ConfigGetParams{},
			buildConfigsMock: func(m *MockconfigResolver) {
				m.EXPECT().BaseConfigID(gomock.Any(), entity.ConfigKindMihomo).Return(int64(7), true, nil)
			},
			buildRoutingMock: func(m *MockmihomoReader) {
				m.EXPECT().Rules(gomock.Any(), int64(7)).Return([]mihomo.RoutingRule{
					{Type: mihomo.RuleMatch, Target: mihomo.PolicyRef{Kind: mihomo.PolicyDirect}},
				}, nil)
				m.EXPECT().ProxyGroups(gomock.Any(), int64(7)).Return(nil, nil)
				m.EXPECT().RuleProviders(gomock.Any(), int64(7)).Return(nil, nil)
				m.EXPECT().Setting(gomock.Any(), int64(7), "base_yaml").Return("mode: rule\n", nil)
				m.EXPECT().Profile(gomock.Any(), int64(7)).Return(mihomo.Profile{Title: "X", Filename: "x.yaml", UpdateInterval: 12}, nil)
			},
			result: &oas.MihomoConfig{
				BaseYAML:              "mode: rule\n",
				Groups:                []oas.MihomoGroup{},
				Rules:                 []oas.MihomoRule{{Type: "MATCH", Target: oas.PolicyRef{Kind: "direct"}}},
				Providers:             []oas.MihomoProvider{},
				ProfileTitle:          "X",
				Filename:              "x.yaml",
				ProfileUpdateInterval: 12,
			},
		},
		{
			// A logical rule surfaces its sub-condition tree on the wire (recursively);
			// a RULE-SET sub-condition's provider id becomes its array index.
			name:   "success.logical_rule",
			params: oas.ConfigGetParams{},
			buildConfigsMock: func(m *MockconfigResolver) {
				m.EXPECT().BaseConfigID(gomock.Any(), entity.ConfigKindMihomo).Return(int64(7), true, nil)
			},
			buildRoutingMock: func(m *MockmihomoReader) {
				m.EXPECT().Rules(gomock.Any(), int64(7)).Return([]mihomo.RoutingRule{
					{Type: mihomo.RuleAnd, Target: mihomo.PolicyRef{Kind: mihomo.PolicyRejectDrop}, Conditions: []mihomo.RuleCondition{
						{Type: mihomo.RuleNetwork, Value: utils.Ptr("UDP")},
						{Type: mihomo.RuleOr, Conditions: []mihomo.RuleCondition{
							{Type: mihomo.RuleDstPort, Value: utils.Ptr("443")},
							{Type: mihomo.RuleRuleSet, ProviderID: utils.Ptr[int64](9)},
						}},
					}},
					{Type: mihomo.RuleMatch, Target: mihomo.PolicyRef{Kind: mihomo.PolicyDirect}},
				}, nil)
				m.EXPECT().ProxyGroups(gomock.Any(), int64(7)).Return(nil, nil)
				m.EXPECT().RuleProviders(gomock.Any(), int64(7)).Return([]mihomo.RuleProvider{{ID: 9, Name: "ads"}}, nil)
				m.EXPECT().Setting(gomock.Any(), int64(7), "base_yaml").Return("", nil)
				m.EXPECT().Profile(gomock.Any(), int64(7)).Return(mihomo.Profile{}, nil)
			},
			result: &oas.MihomoConfig{
				Groups: []oas.MihomoGroup{},
				Rules: []oas.MihomoRule{
					{Type: "AND", Target: oas.PolicyRef{Kind: "reject-drop"}, Conditions: []oas.MihomoCondition{
						{Type: "NETWORK", Value: oas.NewOptString("UDP")},
						{Type: "OR", Conditions: []oas.MihomoCondition{
							{Type: "DST-PORT", Value: oas.NewOptString("443")},
							{Type: "RULE-SET", ProviderIdx: oas.NewOptInt(0)},
						}},
					}},
					{Type: "MATCH", Target: oas.PolicyRef{Kind: "direct"}},
				},
				Providers: []oas.MihomoProvider{{Name: "ads"}},
			},
		},
		{
			// A content read that errors must surface (logged) as a 5xx, not be swallowed.
			name:   "error.read_failed",
			params: oas.ConfigGetParams{},
			buildConfigsMock: func(m *MockconfigResolver) {
				m.EXPECT().BaseConfigID(gomock.Any(), entity.ConfigKindMihomo).Return(int64(7), true, nil)
			},
			buildRoutingMock: func(m *MockmihomoReader) {
				m.EXPECT().Rules(gomock.Any(), int64(7)).Return(nil, internalErr)
			},
			err: internalErr,
		},
		{
			name:   "notfound.user_scope",
			params: oas.ConfigGetParams{User: oas.NewOptInt64(5)},
			buildConfigsMock: func(m *MockconfigResolver) {
				m.EXPECT().UserConfigID(gomock.Any(), int64(5), entity.ConfigKindMihomo).Return(int64(0), false, nil)
			},
			result: &oas.ConfigGetNotFound{},
		},
		{
			name:   "error.internal",
			params: oas.ConfigGetParams{User: oas.NewOptInt64(5)},
			buildConfigsMock: func(m *MockconfigResolver) {
				m.EXPECT().UserConfigID(gomock.Any(), int64(5), entity.ConfigKindMihomo).Return(int64(0), false, internalErr)
			},
			err: internalErr,
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			configs := NewMockconfigResolver(ctrl)
			if tc.buildConfigsMock != nil {
				tc.buildConfigsMock(configs)
			}

			routing := NewMockmihomoReader(ctrl)
			if tc.buildRoutingMock != nil {
				tc.buildRoutingMock(routing)
			}

			res, err := New(configs, routing).ConfigGet(context.Background(), tc.params)

			require.ErrorIs(t, err, tc.err)
			assert.Equal(t, tc.result, res)
		})
	}
}
