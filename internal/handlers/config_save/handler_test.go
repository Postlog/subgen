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
	"github.com/postlog/subgen/internal/utils"
)

func TestHandler_ConfigSave(t *testing.T) {
	internalErr := errors.New("db down")

	// A minimal valid mihomo config: a single MATCH→direct rule, no groups, no
	// providers, empty base YAML. Decode + every Validate* pass on this. userID 0
	// leaves the save scope as the base; >0 sets the per-user scope.
	validReq := func(userID int64) *oas.ConfigSaveReq {
		req := &oas.ConfigSaveReq{
			Rules:                 []oas.MihomoRule{{Type: "MATCH", Target: oas.NewOptPolicyRef(oas.PolicyRef{Kind: "direct"})}},
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

	// The validated config as the saver receives it (a ConfigDraft: group/provider refs
	// carried as array indices; empty groups/providers are empty non-nil slices).
	wantDraft := mihomo.ConfigDraft{
		Rules:     []mihomo.RuleDraft{{Type: mihomo.RuleMatch, Target: &mihomo.RefDraft{Kind: mihomo.PolicyDirect}}},
		Groups:    []mihomo.GroupDraft{},
		Providers: []mihomo.RuleProvider{},
		Profile:   mihomo.Profile{Title: "My VPN", Filename: "my.yaml", UpdateInterval: 6},
	}

	tt := []struct {
		name string
		req  *oas.ConfigSaveReq

		buildConfigsMock func(m *MockconfigsRepo)
		buildRoutingMock func(m *MockroutingRepo)

		result oas.ConfigSaveRes
		err    error
	}{
		{
			name: "success.base",
			req:  validReq(0),
			buildConfigsMock: func(m *MockconfigsRepo) {
				m.EXPECT().EnsureBaseConfigID(gomock.Any(), entity.ConfigKindMihomo).Return(int64(3), nil)
			},
			buildRoutingMock: func(m *MockroutingRepo) {
				m.EXPECT().
					SaveMihomoConfig(gomock.Any(), int64(3), wantDraft).
					Return(nil)
			},
			result: &oas.MessageResponse{Message: MsgSaved},
		},
		{
			// A logical rule decodes its recursive conditions and reaches the saver as a
			// ConfigDraft with the typed sub-condition tree (the wire→draft path end-to-end).
			name: "success.logical",
			req: &oas.ConfigSaveReq{
				Rules: []oas.MihomoRule{
					{Type: "AND", Target: oas.NewOptPolicyRef(oas.PolicyRef{Kind: "reject-drop"}), Children: []oas.MihomoRule{
						{Type: "NETWORK", Value: oas.NewOptString("UDP")},
						{Type: "DST-PORT", Value: oas.NewOptString("443")},
					}},
					{Type: "MATCH", Target: oas.NewOptPolicyRef(oas.PolicyRef{Kind: "direct"})},
				},
				Groups:                []oas.MihomoGroup{},
				Providers:             []oas.MihomoProvider{},
				ProfileTitle:          "My VPN",
				Filename:              "my.yaml",
				ProfileUpdateInterval: 6,
			},
			buildConfigsMock: func(m *MockconfigsRepo) {
				m.EXPECT().EnsureBaseConfigID(gomock.Any(), entity.ConfigKindMihomo).Return(int64(3), nil)
			},
			buildRoutingMock: func(m *MockroutingRepo) {
				m.EXPECT().SaveMihomoConfig(gomock.Any(), int64(3), mihomo.ConfigDraft{
					Rules: []mihomo.RuleDraft{
						{Type: mihomo.RuleAnd, Target: &mihomo.RefDraft{Kind: mihomo.PolicyRejectDrop}, Children: []mihomo.RuleDraft{
							{Type: mihomo.RuleNetwork, Value: utils.Ptr("UDP")},
							{Type: mihomo.RuleDstPort, Value: utils.Ptr("443")},
						}},
						{Type: mihomo.RuleMatch, Target: &mihomo.RefDraft{Kind: mihomo.PolicyDirect}},
					},
					Groups:    []mihomo.GroupDraft{},
					Providers: []mihomo.RuleProvider{},
					Profile:   mihomo.Profile{Title: "My VPN", Filename: "my.yaml", UpdateInterval: 6},
				}).Return(nil)
			},
			result: &oas.MessageResponse{Message: MsgSaved},
		},
		{
			// A malformed logical rule (AND with a single condition) is a 400 with the
			// logical-arity message, before any scope is resolved.
			name: "error.logical_arity",
			req: &oas.ConfigSaveReq{
				Rules: []oas.MihomoRule{
					{Type: "AND", Target: oas.NewOptPolicyRef(oas.PolicyRef{Kind: "direct"}), Children: []oas.MihomoRule{
						{Type: "NETWORK", Value: oas.NewOptString("UDP")},
					}},
					{Type: "MATCH", Target: oas.NewOptPolicyRef(oas.PolicyRef{Kind: "direct"})},
				},
				Groups:                []oas.MihomoGroup{},
				Providers:             []oas.MihomoProvider{},
				ProfileTitle:          "My VPN",
				Filename:              "my.yaml",
				ProfileUpdateInterval: 6,
			},
			result: &oas.ConfigSaveBadRequest{ErrMessage: MsgLogicalArity},
		},
		{
			name: "error.invalid_config",
			req: &oas.ConfigSaveReq{
				// MATCH not last → validation fails before any scope is resolved.
				Rules: []oas.MihomoRule{
					{Type: "MATCH", Target: oas.NewOptPolicyRef(oas.PolicyRef{Kind: "direct"})},
					{Type: "DOMAIN", Value: oas.NewOptString("example.com"), Target: oas.NewOptPolicyRef(oas.PolicyRef{Kind: "direct"})},
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
			buildConfigsMock: func(m *MockconfigsRepo) {
				m.EXPECT().UserConfigID(gomock.Any(), int64(5), entity.ConfigKindMihomo).Return(int64(0), false, nil)
			},
			result: &oas.ConfigSaveBadRequest{ErrMessage: MsgUserConfigMissing},
		},
		{
			name: "error.save_provider_taken",
			req:  validReq(0),
			buildConfigsMock: func(m *MockconfigsRepo) {
				m.EXPECT().EnsureBaseConfigID(gomock.Any(), entity.ConfigKindMihomo).Return(int64(3), nil)
			},
			buildRoutingMock: func(m *MockroutingRepo) {
				m.EXPECT().
					SaveMihomoConfig(gomock.Any(), int64(3), wantDraft).
					Return(entity.ErrRuleProviderNameTaken)
			},
			result: &oas.ConfigSaveBadRequest{ErrMessage: MsgProviderNameTaken},
		},
		{
			name: "error.internal_resolve",
			req:  validReq(0),
			buildConfigsMock: func(m *MockconfigsRepo) {
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

			configs := NewMockconfigsRepo(ctrl)
			if tc.buildConfigsMock != nil {
				tc.buildConfigsMock(configs)
			}

			routing := NewMockroutingRepo(ctrl)
			if tc.buildRoutingMock != nil {
				tc.buildRoutingMock(routing)
			}

			res, err := New(configs, routing).ConfigSave(context.Background(), tc.req)

			require.ErrorIs(t, err, tc.err)
			assert.Equal(t, tc.result, res)
		})
	}
}
