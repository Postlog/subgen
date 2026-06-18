package sublinks

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/mihomo"
	"github.com/postlog/subgen/internal/token"
)

func TestService_Links(t *testing.T) {
	const (
		secret = "secret"
		base   = "http://base/" // trailing slash trimmed by New
	)

	targetErr := errors.New("db down")

	// rawURL/clashmi rebuild the expected values from the same primitives the service
	// uses, so a case asserts the full assembly (token, path, escaping, name) end to end.
	subURL := func(subID string) string {
		return "http://base/sub/mihomo/" + token.Make(secret, subID)
	}
	clashmi := func(subID, name string) string {
		return "clashmi://install-config?url=" + url.QueryEscape(subURL(subID)) +
			"&name=" + url.QueryEscape(name) + "&overwrite=false"
	}

	tt := []struct {
		name      string
		users     []entity.User
		buildMock func(m *MockconfigsRepo, p *MockroutingRepo)
		result    map[int64][]entity.SubLink
		err       error
	}{
		{
			name:   "empty",
			users:  nil,
			result: map[int64][]entity.SubLink{},
		},
		{
			name:  "success.base_only",
			users: []entity.User{{ID: 7, SubID: "s7"}},
			buildMock: func(m *MockconfigsRepo, p *MockroutingRepo) {
				m.EXPECT().BaseConfigID(gomock.Any(), entity.ConfigKindMihomo).Return(int64(100), true, nil)
				p.EXPECT().Profile(gomock.Any(), int64(100)).Return(mihomo.Profile{Title: "Freedom"}, nil)
				m.EXPECT().UserConfigUserIDs(gomock.Any(), entity.ConfigKindMihomo).Return(nil, nil)
			},
			result: map[int64][]entity.SubLink{
				7: {
					{Title: "Mihomo", Value: subURL("s7")},
					{Title: "Clashmi", Value: clashmi("s7", "Freedom")},
				},
			},
		},
		{
			name:  "success.custom_overrides_title",
			users: []entity.User{{ID: 7, SubID: "s7"}, {ID: 8, SubID: "s8"}},
			buildMock: func(m *MockconfigsRepo, p *MockroutingRepo) {
				m.EXPECT().BaseConfigID(gomock.Any(), entity.ConfigKindMihomo).Return(int64(100), true, nil)
				p.EXPECT().Profile(gomock.Any(), int64(100)).Return(mihomo.Profile{Title: "Freedom"}, nil)
				m.EXPECT().UserConfigUserIDs(gomock.Any(), entity.ConfigKindMihomo).Return([]int64{7}, nil)
				m.EXPECT().UserConfigID(gomock.Any(), int64(7), entity.ConfigKindMihomo).Return(int64(200), true, nil)
				p.EXPECT().Profile(gomock.Any(), int64(200)).Return(mihomo.Profile{Title: "Freedom Pro"}, nil)
			},
			result: map[int64][]entity.SubLink{
				7: {
					{Title: "Mihomo", Value: subURL("s7")},
					{Title: "Clashmi", Value: clashmi("s7", "Freedom Pro")},
				},
				8: {
					{Title: "Mihomo", Value: subURL("s8")},
					{Title: "Clashmi", Value: clashmi("s8", "Freedom")},
				},
			},
		},
		{
			name:  "success.no_base_config",
			users: []entity.User{{ID: 7, SubID: "s7"}},
			buildMock: func(m *MockconfigsRepo, p *MockroutingRepo) {
				m.EXPECT().BaseConfigID(gomock.Any(), entity.ConfigKindMihomo).Return(int64(0), false, nil)
				m.EXPECT().UserConfigUserIDs(gomock.Any(), entity.ConfigKindMihomo).Return(nil, nil)
			},
			result: map[int64][]entity.SubLink{
				7: {
					{Title: "Mihomo", Value: subURL("s7")},
					{Title: "Clashmi", Value: clashmi("s7", "")},
				},
			},
		},
		{
			name:  "error.base_config",
			users: []entity.User{{ID: 7, SubID: "s7"}},
			buildMock: func(m *MockconfigsRepo, _ *MockroutingRepo) {
				m.EXPECT().BaseConfigID(gomock.Any(), entity.ConfigKindMihomo).Return(int64(0), false, targetErr)
			},
			err: targetErr,
		},
		{
			name:  "error.profile",
			users: []entity.User{{ID: 7, SubID: "s7"}},
			buildMock: func(m *MockconfigsRepo, p *MockroutingRepo) {
				m.EXPECT().BaseConfigID(gomock.Any(), entity.ConfigKindMihomo).Return(int64(100), true, nil)
				p.EXPECT().Profile(gomock.Any(), int64(100)).Return(mihomo.Profile{}, targetErr)
			},
			err: targetErr,
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			cfgs := NewMockconfigsRepo(ctrl)

			profiles := NewMockroutingRepo(ctrl)
			if tc.buildMock != nil {
				tc.buildMock(cfgs, profiles)
			}

			res, err := New(secret, base, cfgs, profiles).Links(context.Background(), tc.users)

			require.ErrorIs(t, err, tc.err)
			assert.Equal(t, tc.result, res)
		})
	}
}
