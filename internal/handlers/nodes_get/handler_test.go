package nodes_get

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

func TestHandler_NodesGet(t *testing.T) {
	internalErr := errors.New("db down")

	tt := []struct {
		name string

		buildNodesMock func(m *MocknodeLister)

		result oas.NodesGetRes
		err    error
	}{
		{
			name: "success",
			buildNodesMock: func(m *MocknodeLister) {
				m.EXPECT().List(gomock.Any()).Return([]entity.Node{{
					ID:            1,
					Name:          "RU1",
					VPNHost:       "vpn.example",
					PanelBaseURL:  "https://panel.example",
					PanelBasePath: "/app",
					Inbounds: []entity.Inbound{
						{ID: 10, Name: "smart", Port: 8443},
						{ID: 11, Name: "force", Port: 9443},
					},
				}}, nil)
			},
			result: &oas.NodesGetOK{Nodes: []oas.NodesGetOKNodesItem{{
				ID:            1,
				Name:          "RU1",
				VpnHost:       "vpn.example",
				PanelBaseURL:  "https://panel.example",
				PanelBasePath: "/app",
				Inbounds: []oas.NodesGetOKNodesItemInboundsItem{
					{ID: 11, Name: "force", Port: 9443},
					{ID: 10, Name: "smart", Port: 8443},
				},
			}}},
		},
		{
			name: "empty",
			buildNodesMock: func(m *MocknodeLister) {
				m.EXPECT().List(gomock.Any()).Return([]entity.Node{}, nil)
			},
			result: &oas.NodesGetOK{Nodes: []oas.NodesGetOKNodesItem{}},
		},
		{
			name: "error.list",
			buildNodesMock: func(m *MocknodeLister) {
				m.EXPECT().List(gomock.Any()).Return(nil, internalErr)
			},
			err: internalErr,
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			nodes := NewMocknodeLister(ctrl)
			if tc.buildNodesMock != nil {
				tc.buildNodesMock(nodes)
			}

			res, err := New(nodes).NodesGet(context.Background())

			require.ErrorIs(t, err, tc.err)
			assert.Equal(t, tc.result, res)
		})
	}
}
