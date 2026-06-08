package node_save

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/oas"
)

// validCreateReq is a node-create request that passes web.ValidateNode.
func validCreateReq() *oas.NodeSaveReq {
	return &oas.NodeSaveReq{
		Name:          "RU1",
		VpnHost:       "host.example",
		PanelBaseURL:  "https://panel.example:8443",
		PanelBasePath: "/",
		Inbounds:      []oas.NodeSaveReqInboundsItem{{Name: "smart", Port: 8443}},
	}
}

func TestHandler_NodeSave(t *testing.T) {
	internalErr := errors.New("db down")

	tt := []struct {
		name string
		req  *oas.NodeSaveReq

		buildNodeMock    func(m *MocknodeRepo)
		buildRoutingMock func(m *MockroutingRepo)

		result oas.NodeSaveRes
		err    error

		// assertRes, when set, replaces the exact-result comparison (used for the
		// interpolated validation message).
		assertRes func(t *testing.T, res oas.NodeSaveRes)
	}{
		{
			name: "success.create",
			req:  validCreateReq(),
			buildNodeMock: func(m *MocknodeRepo) {
				m.EXPECT().Create(gomock.Any(), gomock.Any()).Return(int64(1), nil)
			},
			result: &oas.MessageResponse{Message: "Узел сохранён: RU1"},
		},
		{
			name: "error.invalid",
			req: &oas.NodeSaveReq{
				Name:          "RU1",
				VpnHost:       "http://bad:8080",
				PanelBaseURL:  "https://panel.example:8443",
				PanelBasePath: "/",
				Inbounds:      []oas.NodeSaveReqInboundsItem{{Name: "smart", Port: 8443}},
			},
			assertRes: func(t *testing.T, res oas.NodeSaveRes) {
				bad, ok := res.(*oas.NodeSaveBadRequest)
				require.True(t, ok, "want *oas.NodeSaveBadRequest, got %T", res)
				assert.Contains(t, bad.ErrMessage, "невалиден")
			},
		},
		{
			name: "error.name_taken",
			req:  validCreateReq(),
			buildNodeMock: func(m *MocknodeRepo) {
				m.EXPECT().Create(gomock.Any(), gomock.Any()).Return(int64(0), entity.ErrNodeNameTaken)
			},
			result: &oas.NodeSaveConflict{ErrMessage: msgNodeNameTaken},
		},
		{
			name: "error.internal",
			req:  validCreateReq(),
			buildNodeMock: func(m *MocknodeRepo) {
				m.EXPECT().Create(gomock.Any(), gomock.Any()).Return(int64(0), internalErr)
			},
			err: internalErr,
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			nodes := NewMocknodeRepo(ctrl)
			if tc.buildNodeMock != nil {
				tc.buildNodeMock(nodes)
			}

			routing := NewMockroutingRepo(ctrl)
			if tc.buildRoutingMock != nil {
				tc.buildRoutingMock(routing)
			}

			res, err := New(nodes, routing).NodeSave(context.Background(), tc.req)

			require.ErrorIs(t, err, tc.err)

			if tc.assertRes != nil {
				tc.assertRes(t, res)
				return
			}

			assert.Equal(t, tc.result, res)
		})
	}
}
