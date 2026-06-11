package node_delete

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

// node7 is a one-inbound node: "main" :4433 (inbound id 10).
func node7() *entity.Node {
	return &entity.Node{
		ID: 7, Name: "N1",
		Inbounds: []entity.Inbound{{ID: 10, Name: "main", Port: 4433}},
	}
}

type mocks struct {
	nodes   *MocknodeRepo
	routing *MockroutingRepo
}

func TestHandler_NodeDelete(t *testing.T) {
	targetErr := errors.New("test")

	tt := []struct {
		name       string
		reqID      int64
		buildMocks func(m *mocks)
		wantBad    bool  // expect a 400 (NodeDeleteBadRequest)
		wantErr    error // expect a propagated error (-> 500)
	}{
		{
			name:    "error.invalid_id",
			reqID:   0,
			wantBad: true, // guard returns before any repo call
		},
		{
			name:    "error.users_blocking",
			reqID:   7,
			wantBad: true,
			buildMocks: func(m *mocks) {
				m.nodes.EXPECT().Get(gomock.Any(), int64(7)).Return(node7(), nil)
				m.nodes.EXPECT().ConnectionCountsByInbound(gomock.Any(), []int64{10}).Return(map[int64]int{10: 2}, nil)
				m.routing.EXPECT().InboundRefCounts(gomock.Any(), []int64{10}).Return(map[int64]int{}, nil)
			},
		},
		{
			name:    "error.delete",
			reqID:   7,
			wantErr: targetErr,
			buildMocks: func(m *mocks) {
				m.nodes.EXPECT().Get(gomock.Any(), int64(7)).Return(node7(), nil)
				m.nodes.EXPECT().ConnectionCountsByInbound(gomock.Any(), []int64{10}).Return(map[int64]int{}, nil)
				m.routing.EXPECT().InboundRefCounts(gomock.Any(), []int64{10}).Return(map[int64]int{}, nil)
				m.nodes.EXPECT().Delete(gomock.Any(), int64(7)).Return(targetErr)
			},
		},
		{
			name:  "success",
			reqID: 7,
			buildMocks: func(m *mocks) {
				m.nodes.EXPECT().Get(gomock.Any(), int64(7)).Return(node7(), nil)
				m.nodes.EXPECT().ConnectionCountsByInbound(gomock.Any(), []int64{10}).Return(map[int64]int{}, nil)
				m.routing.EXPECT().InboundRefCounts(gomock.Any(), []int64{10}).Return(map[int64]int{}, nil)
				m.nodes.EXPECT().Delete(gomock.Any(), int64(7)).Return(nil)
			},
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			m := &mocks{nodes: NewMocknodeRepo(ctrl), routing: NewMockroutingRepo(ctrl)}
			if tc.buildMocks != nil {
				tc.buildMocks(m)
			}

			res, err := New(m.nodes, m.routing).NodeDelete(context.Background(), &oas.NodeDeleteReq{ID: tc.reqID})

			require.ErrorIs(t, err, tc.wantErr)

			switch {
			case tc.wantErr != nil:
				assert.Nil(t, res)
			case tc.wantBad:
				bad, ok := res.(*oas.NodeDeleteBadRequest)
				require.True(t, ok, "want NodeDeleteBadRequest, got %T", res)
				assert.NotEmpty(t, bad.ErrMessage)
			default:
				ok, _ := res.(*oas.MessageResponse)
				require.NotNil(t, ok, "want MessageResponse, got %T", res)
				assert.Equal(t, msgDeleted, ok.Message)
			}
		})
	}
}
