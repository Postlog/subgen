package provisioning

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/postlog/subgen/internal/entity"
)

var ctx = context.Background()

// mocks bundles the service's dependency mocks for a test case.
type mocks struct {
	users  *MockuserRepo
	nodes  *MocknodeRepo
	client *MockpanelClient
	cache  *MockfleetCache
}

func newMocks(ctrl *gomock.Controller) *mocks {
	return &mocks{
		users:  NewMockuserRepo(ctrl),
		nodes:  NewMocknodeRepo(ctrl),
		client: NewMockpanelClient(ctrl),
		cache:  NewMockfleetCache(ctrl),
	}
}

// n1 is a one-node registry: node id 1 "N1" with inbound id 10 ("a" :4433) and
// inbound id 11 ("b" :8443).
func n1() []entity.Node {
	return []entity.Node{{
		ID: 1, Name: "N1", VPNHost: "n1", PanelBaseURL: "u", PanelBasePath: "/", Token: "t",
		Inbounds: []entity.Inbound{
			{ID: 10, Name: "a", Port: 4433},
			{ID: 11, Name: "b", Port: 8443},
		},
	}}
}

// panelInbounds is what the N1 panel returns: the two 3x-ui inbounds with their
// (panel-side) ids, keyed by port.
func panelInbounds() []entity.PanelInbound {
	return []entity.PanelInbound{
		{ID: 1, Port: 4433, Enable: true},
		{ID: 2, Port: 8443, Enable: true},
	}
}

func TestService_CreateUser(t *testing.T) {
	t.Parallel()

	targetErr := errors.New("test")

	tt := []struct {
		name       string
		inName     string
		sel        entity.ConnectionSelection
		buildMocks func(m *mocks)
		wantConns  int
		err        error // sentinel/target to ErrorIs; nil => success
	}{
		{
			name: "error.no_connection", inName: "postlog", err: entity.ErrNoConnectionSelected,
		},
		{
			name: "error.invalid_name", inName: "Bad Name",
			sel: entity.ConnectionSelection{InboundIDs: []int64{10}}, err: entity.ErrInvalidUserName,
		},
		{
			// Name uniqueness now comes from the users.name constraint: Create returns
			// entity.ErrNameTaken (no pre-check). The service propagates it.
			name: "error.name_taken", inName: "postlog", sel: entity.ConnectionSelection{InboundIDs: []int64{10}},
			err: entity.ErrNameTaken,
			buildMocks: func(m *mocks) {
				m.nodes.EXPECT().List(gomock.Any()).Return(n1(), nil)
				m.client.EXPECT().ListInbounds(gomock.Any(), gomock.Any()).Return(panelInbounds(), nil)
				m.users.EXPECT().Create(gomock.Any(), gomock.Any()).Return(entity.ErrNameTaken)
			},
		},
		{
			name: "error.unknown_inbound", inName: "postlog", sel: entity.ConnectionSelection{InboundIDs: []int64{999}},
			err: entity.ErrInboundNotFound,
			buildMocks: func(m *mocks) {
				m.nodes.EXPECT().List(gomock.Any()).Return(n1(), nil)
			},
		},
		{
			name: "error.create_repo", inName: "postlog", sel: entity.ConnectionSelection{InboundIDs: []int64{10}},
			err: targetErr,
			buildMocks: func(m *mocks) {
				m.nodes.EXPECT().List(gomock.Any()).Return(n1(), nil)
				// pre-flight email check: panel is free
				m.client.EXPECT().ListInbounds(gomock.Any(), gomock.Any()).Return(panelInbounds(), nil)
				m.users.EXPECT().Create(gomock.Any(), gomock.Any()).Return(targetErr)
			},
		},
		{
			// A client with our email (nickname) already exists on a target panel —
			// orphan, manual, or half-finished delete. Create must ABORT, touching
			// nothing: no store row, no DelClient, no AddClient.
			name: "error.email_exists_on_panel", inName: "postlog",
			sel: entity.ConnectionSelection{InboundIDs: []int64{10}},
			err: entity.PanelClientExistsError{Node: "N1"},
			buildMocks: func(m *mocks) {
				m.nodes.EXPECT().List(gomock.Any()).Return(n1(), nil)
				m.client.EXPECT().ListInbounds(gomock.Any(), gomock.Any()).Return([]entity.PanelInbound{
					{ID: 1, Port: 4433, Enable: true, Clients: []entity.PanelClient{{Email: "postlog", UUID: uuid.New()}}},
				}, nil)
				// no Create, no DelClient, no AddClient, no Invalidate
			},
		},
		{
			name: "success.two_inbounds_same_node", inName: "postlog",
			sel:       entity.ConnectionSelection{InboundIDs: []int64{10, 11}},
			wantConns: 2,
			buildMocks: func(m *mocks) {
				m.nodes.EXPECT().List(gomock.Any()).Return(n1(), nil)
				m.users.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
				// ListInbounds twice: pre-flight email check + syncPanels lookup; the
				// panel has no "postlog" client → free.
				m.client.EXPECT().ListInbounds(gomock.Any(), gomock.Any()).Return(panelInbounds(), nil).Times(2)
				// one panel, two inbounds (4433+8443) → single add with both 3x-ui ids.
				m.client.EXPECT().AddClient(gomock.Any(), gomock.Any(), []int{1, 2}, gomock.Any()).Return(nil)
				m.cache.EXPECT().Invalidate()
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			m := newMocks(ctrl)
			if tc.buildMocks != nil {
				tc.buildMocks(m)
			}

			svc := New(m.users, m.nodes, m.client, m.cache)
			u, err := svc.CreateUser(ctx, tc.inName, tc.sel)

			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, u.Connections, tc.wantConns)
			assert.NotEmpty(t, u.SubID)
		})
	}
}

func TestService_DeleteUser(t *testing.T) {
	t.Parallel()

	user := &entity.User{ID: 7, Name: "postlog", SubID: "s", Connections: []entity.Connection{
		{NodeID: 1, Node: "N1", Name: "a", Port: 4433, InboundID: 10},
	}}

	tt := []struct {
		name       string
		buildMocks func(m *mocks)
	}{
		{
			name: "success",
			buildMocks: func(m *mocks) {
				m.users.EXPECT().Get(gomock.Any(), int64(7)).Return(user, nil)
				m.nodes.EXPECT().List(gomock.Any()).Return(n1(), nil)
				m.client.EXPECT().DelClient(gomock.Any(), gomock.Any(), "postlog").Return(nil)
				m.cache.EXPECT().Invalidate()
				m.users.EXPECT().Delete(gomock.Any(), int64(7)).Return(nil)
			},
		},
		{
			name: "success.node_absent_still_deletes_row",
			buildMocks: func(m *mocks) {
				m.users.EXPECT().Get(gomock.Any(), int64(7)).Return(user, nil)
				m.nodes.EXPECT().List(gomock.Any()).Return(nil, nil) // node gone from registry → skip del
				m.cache.EXPECT().Invalidate()
				m.users.EXPECT().Delete(gomock.Any(), int64(7)).Return(nil)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			m := newMocks(ctrl)
			tc.buildMocks(m)
			svc := New(m.users, m.nodes, m.client, m.cache)
			require.NoError(t, svc.DeleteUser(ctx, 7))
		})
	}
}

func TestService_sameInboundIDs(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name string
		a, b []int64
		want bool
	}{
		{name: "equal_unordered", a: []int64{1, 2}, b: []int64{2, 1}, want: true},
		{name: "different_len", a: []int64{1}, b: []int64{1, 2}, want: false},
		{name: "different_values", a: []int64{1, 2}, b: []int64{1, 3}, want: false},
		{name: "both_empty", a: nil, b: nil, want: true},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, sameInboundIDs(tc.a, tc.b))
		})
	}
}
