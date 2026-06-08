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
			},
			result: &oas.MihomoConfig{
				BaseYAML:  "mode: rule\n",
				Groups:    []oas.MihomoGroup{},
				Rules:     []oas.MihomoRule{{Type: "MATCH", Target: oas.PolicyRef{Kind: "direct"}}},
				Providers: []oas.MihomoProvider{},
			},
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
