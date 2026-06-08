package sub

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/oas"
	"github.com/postlog/subgen/internal/token"
)

const testSecret = "hmac-secret"

type mocks struct {
	users    *MockuserResolver
	fleet    *MockfleetReader
	configs  *MockconfigResolver
	renderer *MockEngineRenderer
}

func TestHandler_Sub(t *testing.T) {
	internalErr := errors.New("db down")

	matchingToken := token.Make(testSecret, "sub1")

	tt := []struct {
		name   string
		params oas.SubParams

		buildMocks func(m *mocks)

		err error

		assertRes func(t *testing.T, res oas.SubRes)
	}{
		{
			name:   "notfound.unknown_kind",
			params: oas.SubParams{Kind: "nope", Token: "x"},
			assertRes: func(t *testing.T, res oas.SubRes) {
				_, ok := res.(*oas.SubNotFound)
				require.True(t, ok, "want *oas.SubNotFound, got %T", res)
			},
		},
		{
			name:   "notfound.unmatched_token",
			params: oas.SubParams{Kind: "mihomo", Token: "deadbeefdeadbeefdeadbeef"},
			buildMocks: func(m *mocks) {
				m.users.EXPECT().SubIDs(gomock.Any()).Return([]string{"sub1"}, nil)
			},
			assertRes: func(t *testing.T, res oas.SubRes) {
				_, ok := res.(*oas.SubNotFound)
				require.True(t, ok, "want *oas.SubNotFound, got %T", res)
			},
		},
		{
			name:   "success",
			params: oas.SubParams{Kind: "mihomo", Token: matchingToken},
			buildMocks: func(m *mocks) {
				m.users.EXPECT().SubIDs(gomock.Any()).Return([]string{"sub1"}, nil)
				m.users.EXPECT().IDBySubID(gomock.Any(), "sub1").Return(int64(7), nil)
				m.configs.EXPECT().UserConfigID(gomock.Any(), int64(7), entity.ConfigKindMihomo).Return(int64(0), false, nil)
				m.configs.EXPECT().BaseConfigID(gomock.Any(), entity.ConfigKindMihomo).Return(int64(3), true, nil)
				m.fleet.EXPECT().Fleet(gomock.Any()).Return(&entity.Fleet{
					Subs: map[string]*entity.Subscriber{
						"sub1": {SubID: "sub1", Up: 10, Down: 20, Total: 100, Expiry: 5000},
					},
				}, nil)
				m.renderer.EXPECT().
					Render(gomock.Any(), &entity.Subscriber{SubID: "sub1", Up: 10, Down: 20, Total: 100, Expiry: 5000}, int64(3)).
					Return([]byte("yaml"), RenderMeta{Filename: "f.yaml"}, nil)
			},
			assertRes: func(t *testing.T, res oas.SubRes) {
				ok, isOK := res.(*oas.SubOKHeaders)
				require.True(t, isOK, "want *oas.SubOKHeaders, got %T", res)

				assert.Equal(t, "300", ok.ProfileUpdateInterval.Value)
				assert.Equal(t, "base64:"+base64.StdEncoding.EncodeToString([]byte("Profile")), ok.ProfileTitle.Value)
				assert.Equal(t, `attachment; filename="f.yaml"`, ok.ContentDisposition.Value)
				assert.Equal(t, "upload=10; download=20; total=100; expire=5", ok.SubscriptionUserinfo.Value)

				body, err := io.ReadAll(ok.Response.Data)
				require.NoError(t, err)
				assert.Equal(t, "yaml", string(body))
			},
		},
		{
			name:   "error.subids",
			params: oas.SubParams{Kind: "mihomo", Token: matchingToken},
			buildMocks: func(m *mocks) {
				m.users.EXPECT().SubIDs(gomock.Any()).Return(nil, internalErr)
			},
			err: internalErr,
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			m := &mocks{
				users:    NewMockuserResolver(ctrl),
				fleet:    NewMockfleetReader(ctrl),
				configs:  NewMockconfigResolver(ctrl),
				renderer: NewMockEngineRenderer(ctrl),
			}
			if tc.buildMocks != nil {
				tc.buildMocks(m)
			}

			renderers := map[entity.ConfigKind]EngineRenderer{entity.ConfigKindMihomo: m.renderer}

			res, err := New(m.users, m.fleet, m.configs, renderers, testSecret, "Profile", 300).
				Sub(context.Background(), tc.params)

			require.ErrorIs(t, err, tc.err)

			if tc.assertRes != nil {
				tc.assertRes(t, res)
				return
			}

			assert.Nil(t, res)
		})
	}
}
