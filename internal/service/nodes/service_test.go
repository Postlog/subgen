package nodes

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/postlog/subgen/internal/entity"
)

var ctx = context.Background()

// validNode is a node that passes validateNode: one inbound "force" :8443 (id 10).
func validNode() entity.Node {
	return entity.Node{
		Name: "RU1", VPNHost: "1.2.3.4", PanelBaseURL: "https://1.2.3.4:2096", PanelBasePath: "/p/",
		Inbounds: []entity.Inbound{{ID: 10, Name: "force", Port: 8443}},
	}
}

type mocks struct {
	nodes   *MocknodeRepo
	routing *MockroutingRepo
}

func newMocks(ctrl *gomock.Controller) *mocks {
	return &mocks{nodes: NewMocknodeRepo(ctrl), routing: NewMockroutingRepo(ctrl)}
}

func TestService_Save(t *testing.T) {
	targetErr := errors.New("store")

	tt := []struct {
		name       string
		node       entity.Node
		buildMocks func(m *mocks)
		wantID     int64
		err        error // sentinel/target to ErrorIs; nil => success
		wantErr    bool  // for InboundsBlockedError (errors.As, not a sentinel)
	}{
		{
			name: "error.invalid_node", node: func() entity.Node { n := validNode(); n.Name = "bad.name"; return n }(),
			err: entity.ErrValidationNodeName, // no repo calls
		},
		{
			name: "success.create", node: validNode(), wantID: 5,
			buildMocks: func(m *mocks) {
				m.nodes.EXPECT().Create(gomock.Any(), gomock.Any()).Return(int64(5), nil)
			},
		},
		{
			name: "error.create_conflict", node: validNode(),
			err: entity.ErrNodeNameTaken,
			buildMocks: func(m *mocks) {
				m.nodes.EXPECT().Create(gomock.Any(), gomock.Any()).Return(int64(0), entity.ErrNodeNameTaken)
			},
		},
		{
			name: "success.update", node: func() entity.Node { n := validNode(); n.ID = 7; return n }(), wantID: 7,
			buildMocks: func(m *mocks) {
				// current node has the same inbound → nothing removed → no block check.
				cur := &entity.Node{ID: 7, Name: "RU1", Inbounds: []entity.Inbound{{ID: 10, Name: "force", Port: 8443}}}
				m.nodes.EXPECT().Get(gomock.Any(), int64(7)).Return(cur, nil)
				m.nodes.EXPECT().Update(gomock.Any(), int64(7), gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name: "error.update_blocked", node: func() entity.Node { n := validNode(); n.ID = 7; return n }(),
			wantErr: true, // dropping inbound 11, still referenced → InboundsBlockedError
			buildMocks: func(m *mocks) {
				// current has an extra inbound 11 absent from the submission → "removed".
				cur := &entity.Node{ID: 7, Name: "RU1", Inbounds: []entity.Inbound{
					{ID: 10, Name: "force", Port: 8443}, {ID: 11, Name: "alt", Port: 9000},
				}}
				m.nodes.EXPECT().Get(gomock.Any(), int64(7)).Return(cur, nil)
				// blocked() re-reads the node, then counts.
				m.nodes.EXPECT().Get(gomock.Any(), int64(7)).Return(cur, nil)
				m.nodes.EXPECT().ConnectionCountsByInbound(gomock.Any(), []int64{11}).Return(map[int64]int{11: 3}, nil)
				m.routing.EXPECT().InboundRefCounts(gomock.Any(), []int64{11}).Return(map[int64]int{}, nil)
			},
		},
		{
			name: "error.update_repo", node: func() entity.Node { n := validNode(); n.ID = 7; return n }(),
			err: targetErr,
			buildMocks: func(m *mocks) {
				cur := &entity.Node{ID: 7, Name: "RU1", Inbounds: []entity.Inbound{{ID: 10, Name: "force", Port: 8443}}}
				m.nodes.EXPECT().Get(gomock.Any(), int64(7)).Return(cur, nil)
				m.nodes.EXPECT().Update(gomock.Any(), int64(7), gomock.Any(), gomock.Any()).Return(targetErr)
			},
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			m := newMocks(ctrl)
			if tc.buildMocks != nil {
				tc.buildMocks(m)
			}

			id, err := New(m.nodes, m.routing).Save(ctx, tc.node)

			if tc.wantErr {
				var blocked entity.InboundsBlockedError
				require.ErrorAs(t, err, &blocked)
				assert.NotEmpty(t, blocked.Inbounds)

				return
			}

			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantID, id)
		})
	}
}

func TestService_Delete(t *testing.T) {
	node := &entity.Node{ID: 7, Name: "RU1", Inbounds: []entity.Inbound{{ID: 10, Name: "force", Port: 8443}}}

	tt := []struct {
		name       string
		buildMocks func(m *mocks)
		wantErr    bool // expect InboundsBlockedError
		err        error
	}{
		{
			name: "success",
			buildMocks: func(m *mocks) {
				m.nodes.EXPECT().Get(gomock.Any(), int64(7)).Return(node, nil)
				m.nodes.EXPECT().ConnectionCountsByInbound(gomock.Any(), []int64{10}).Return(map[int64]int{}, nil)
				m.routing.EXPECT().InboundRefCounts(gomock.Any(), []int64{10}).Return(map[int64]int{}, nil)
				m.nodes.EXPECT().Delete(gomock.Any(), int64(7)).Return(nil)
			},
		},
		{
			name:    "error.blocked",
			wantErr: true,
			buildMocks: func(m *mocks) {
				m.nodes.EXPECT().Get(gomock.Any(), int64(7)).Return(node, nil)
				m.nodes.EXPECT().ConnectionCountsByInbound(gomock.Any(), []int64{10}).Return(map[int64]int{}, nil)
				m.routing.EXPECT().InboundRefCounts(gomock.Any(), []int64{10}).Return(map[int64]int{10: 2}, nil)
			},
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			m := newMocks(ctrl)
			if tc.buildMocks != nil {
				tc.buildMocks(m)
			}

			err := New(m.nodes, m.routing).Delete(ctx, 7)

			if tc.wantErr {
				var blocked entity.InboundsBlockedError
				require.ErrorAs(t, err, &blocked)

				return
			}

			require.ErrorIs(t, err, tc.err)
		})
	}
}
