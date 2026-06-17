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

// validNode is a node that passes validateNode: one inbound "force" :8443 (id 10). Its
// base path already starts with "/", so validateNode does not mutate it — the value the
// service forwards to the repo equals validNode() (mocks assert that exact node).
func validNode() entity.Node {
	return entity.Node{
		Name: "RU1", VPNHost: "1.2.3.4", PanelBaseURL: "https://1.2.3.4:2096", PanelBasePath: "/p/",
		Inbounds: []entity.Inbound{{ID: 10, Name: "force", Port: 8443}},
	}
}

// updateNode is validNode with an id set — an update rather than a create. Token is empty,
// so the service calls Update with setToken=false.
func updateNode() entity.Node { n := validNode(); n.ID = 7; return n }

func TestService_Save(t *testing.T) {
	targetErr := errors.New("store")

	tt := []struct {
		name      string
		node      entity.Node
		buildMock func(m *MocknodesRepo)
		wantID    int64
		err       error // sentinel/target to ErrorIs; nil => success
	}{
		{
			name: "error.invalid_node", node: func() entity.Node { n := validNode(); n.Name = "bad.name"; return n }(),
			err: entity.ErrValidationNodeName, // no repo calls
		},
		{
			name: "success.create", node: validNode(), wantID: 5,
			buildMock: func(m *MocknodesRepo) {
				m.EXPECT().Create(gomock.Any(), validNode()).Return(int64(5), nil)
			},
		},
		{
			name: "error.create_conflict", node: validNode(),
			err: entity.ErrNodeNameTaken,
			buildMock: func(m *MocknodesRepo) {
				m.EXPECT().Create(gomock.Any(), validNode()).Return(int64(0), entity.ErrNodeNameTaken)
			},
		},
		{
			name: "success.update", node: updateNode(), wantID: 7,
			buildMock: func(m *MocknodesRepo) {
				m.EXPECT().Update(gomock.Any(), int64(7), updateNode(), false).Return(nil)
			},
		},
		{
			name: "error.update_referenced", node: updateNode(),
			err: entity.ErrInboundReferenced, // FK refused a dropped inbound — propagated
			buildMock: func(m *MocknodesRepo) {
				m.EXPECT().Update(gomock.Any(), int64(7), updateNode(), false).Return(entity.ErrInboundReferenced)
			},
		},
		{
			name: "error.update_repo", node: updateNode(),
			err: targetErr,
			buildMock: func(m *MocknodesRepo) {
				m.EXPECT().Update(gomock.Any(), int64(7), updateNode(), false).Return(targetErr)
			},
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			m := NewMocknodesRepo(ctrl)
			if tc.buildMock != nil {
				tc.buildMock(m)
			}

			id, err := New(m).Save(ctx, tc.node)

			require.ErrorIs(t, err, tc.err)
			assert.Equal(t, tc.wantID, id)
		})
	}
}

func TestService_Delete(t *testing.T) {
	targetErr := errors.New("store")

	tt := []struct {
		name      string
		buildMock func(m *MocknodesRepo)
		err       error
	}{
		{
			name: "success",
			buildMock: func(m *MocknodesRepo) {
				m.EXPECT().Delete(gomock.Any(), int64(7)).Return(nil)
			},
		},
		{
			name: "error.not_found",
			err:  entity.ErrNodeNotFound, // repo found no row to delete — propagated
			buildMock: func(m *MocknodesRepo) {
				m.EXPECT().Delete(gomock.Any(), int64(7)).Return(entity.ErrNodeNotFound)
			},
		},
		{
			name: "error.referenced",
			err:  entity.ErrInboundReferenced, // FK refused a referenced inbound — propagated
			buildMock: func(m *MocknodesRepo) {
				m.EXPECT().Delete(gomock.Any(), int64(7)).Return(entity.ErrInboundReferenced)
			},
		},
		{
			name: "error.repo",
			err:  targetErr,
			buildMock: func(m *MocknodesRepo) {
				m.EXPECT().Delete(gomock.Any(), int64(7)).Return(targetErr)
			},
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			m := NewMocknodesRepo(ctrl)
			if tc.buildMock != nil {
				tc.buildMock(m)
			}

			err := New(m).Delete(ctx, 7)

			require.ErrorIs(t, err, tc.err)
		})
	}
}
