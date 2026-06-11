package config_save

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

func TestHandler_ConfigSave(t *testing.T) {
	internalErr := errors.New("db down")

	// A minimal valid mihomo config: a single MATCH→direct rule, no groups, no
	// providers, empty base YAML. Decode + every Validate* pass on this. userID 0
	// leaves the save scope as the base; >0 sets the per-user scope.
	validReq := func(userID int64) *oas.ConfigSaveReq {
		req := &oas.ConfigSaveReq{
			Rules:                 []oas.MihomoRule{{Type: "MATCH", Target: oas.PolicyRef{Kind: "direct"}}},
			Groups:                []oas.MihomoGroup{},
			Providers:             []oas.MihomoProvider{},
			ProfileTitle:          "My VPN",
			Filename:              "my.yaml",
			ProfileUpdateInterval: 6,
		}
		if userID != 0 {
			req.UserId = oas.NewOptInt64(userID)
		}

		return req
	}

	// The validated config as the saver receives it.
	wantRules := []mihomo.RoutingRule{{Type: mihomo.RuleMatch, Target: mihomo.PolicyRef{Kind: mihomo.PolicyDirect}}}
	wantGroups := []mihomo.ProxyGroup{}
	wantProvs := []mihomo.RuleProvider(nil)
	wantProfile := mihomo.Profile{Title: "My VPN", Filename: "my.yaml", UpdateInterval: 6}

	tt := []struct {
		name string
		req  *oas.ConfigSaveReq

		buildConfigsMock func(m *MockconfigResolver)
		buildRoutingMock func(m *MockmihomoSaver)

		result oas.ConfigSaveRes
		err    error
	}{
		{
			name: "success.base",
			req:  validReq(0),
			buildConfigsMock: func(m *MockconfigResolver) {
				m.EXPECT().EnsureBaseConfigID(gomock.Any(), entity.ConfigKindMihomo).Return(int64(3), nil)
			},
			buildRoutingMock: func(m *MockmihomoSaver) {
				m.EXPECT().
					SaveMihomoConfig(gomock.Any(), int64(3), wantRules, wantGroups, wantProvs, "", wantProfile).
					Return(nil)
			},
			result: &oas.MessageResponse{Message: MsgSaved},
		},
		{
			name: "error.invalid_config",
			req: &oas.ConfigSaveReq{
				// MATCH not last → validation fails before any scope is resolved.
				Rules: []oas.MihomoRule{
					{Type: "MATCH", Target: oas.PolicyRef{Kind: "direct"}},
					{Type: "DOMAIN", Value: "example.com", Target: oas.PolicyRef{Kind: "direct"}},
				},
				Groups:    []oas.MihomoGroup{},
				Providers: []oas.MihomoProvider{},
			},
			result: &oas.ConfigSaveBadRequest{ErrMessage: MsgMatchNotLast},
		},
		{
			// Profile is validated last: a valid config with an empty title is rejected.
			name:   "error.profile_title_empty",
			req:    func() *oas.ConfigSaveReq { r := validReq(0); r.ProfileTitle = ""; return r }(),
			result: &oas.ConfigSaveBadRequest{ErrMessage: MsgProfileTitleEmpty},
		},
		{
			name:   "error.profile_filename_empty",
			req:    func() *oas.ConfigSaveReq { r := validReq(0); r.Filename = ""; return r }(),
			result: &oas.ConfigSaveBadRequest{ErrMessage: MsgProfileFilenameEmpty},
		},
		{
			name:   "error.profile_filename_invalid",
			req:    func() *oas.ConfigSaveReq { r := validReq(0); r.Filename = "sub/dir.yaml"; return r }(),
			result: &oas.ConfigSaveBadRequest{ErrMessage: MsgProfileFilenameInvalid},
		},
		{
			name:   "error.profile_interval_invalid",
			req:    func() *oas.ConfigSaveReq { r := validReq(0); r.ProfileUpdateInterval = 0; return r }(),
			result: &oas.ConfigSaveBadRequest{ErrMessage: MsgProfileIntervalInvalid},
		},
		{
			name: "error.user_config_not_found",
			req:  validReq(5),
			buildConfigsMock: func(m *MockconfigResolver) {
				m.EXPECT().UserConfigID(gomock.Any(), int64(5), entity.ConfigKindMihomo).Return(int64(0), false, nil)
			},
			result: &oas.ConfigSaveBadRequest{ErrMessage: MsgUserConfigMissing},
		},
		{
			name: "error.save_provider_taken",
			req:  validReq(0),
			buildConfigsMock: func(m *MockconfigResolver) {
				m.EXPECT().EnsureBaseConfigID(gomock.Any(), entity.ConfigKindMihomo).Return(int64(3), nil)
			},
			buildRoutingMock: func(m *MockmihomoSaver) {
				m.EXPECT().
					SaveMihomoConfig(gomock.Any(), int64(3), wantRules, wantGroups, wantProvs, "", wantProfile).
					Return(entity.ErrRuleProviderNameTaken)
			},
			result: &oas.ConfigSaveBadRequest{ErrMessage: MsgProviderNameTaken},
		},
		{
			name: "error.internal_resolve",
			req:  validReq(0),
			buildConfigsMock: func(m *MockconfigResolver) {
				m.EXPECT().EnsureBaseConfigID(gomock.Any(), entity.ConfigKindMihomo).Return(int64(0), internalErr)
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

			routing := NewMockmihomoSaver(ctrl)
			if tc.buildRoutingMock != nil {
				tc.buildRoutingMock(routing)
			}

			res, err := New(configs, routing).ConfigSave(context.Background(), tc.req)

			require.ErrorIs(t, err, tc.err)
			assert.Equal(t, tc.result, res)
		})
	}
}
