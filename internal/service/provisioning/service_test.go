package provisioning

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/utils"
)

var ctx = context.Background()

// Deterministic random-id sources: the test overrides Service.genID/genUUID with these so
// the *entity.User and entity.ClientSpec the service builds are fully pinned (no matchers).
const fixedSubID = "subid000000000000"

var fixedClientID = uuid.MustParse("11111111-1111-1111-1111-111111111111")

// newService builds the provisioning service with its random-id sources pinned, so panel
// calls and users.Create can be asserted exactly.
func newService(m *mocks) *Service {
	s := New(m.users, m.nodes, m.client)
	s.genID = func(int) string { return fixedSubID }
	s.genUUID = func() uuid.UUID { return fixedClientID }

	return s
}

// n1Target is the per-call panel credentials the service derives from node N1
// (target(n) = {BaseURL, BasePath, Token}). Deterministic, so panel calls assert it
// exactly rather than gomock.Any().
func n1Target() entity.PanelTarget {
	return entity.PanelTarget{BaseURL: "u", BasePath: "/", Token: "t"}
}

// mocks bundles the service's dependency mocks for a test case.
type mocks struct {
	users  *MockusersRepo
	nodes  *MocknodesRepo
	client *MockpanelClient
}

func newMocks(ctrl *gomock.Controller) *mocks {
	return &mocks{
		users:  NewMockusersRepo(ctrl),
		nodes:  NewMocknodesRepo(ctrl),
		client: NewMockpanelClient(ctrl),
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
		in         entity.UserCreateParams
		buildMocks func(m *mocks)
		wantConns  int
		err        error // sentinel/target to ErrorIs; nil => success
	}{
		{
			name: "error.no_connection", in: entity.UserCreateParams{Name: "postlog"}, err: entity.ErrNoConnectionSelected,
		},
		{
			name: "error.invalid_name",
			in:   entity.UserCreateParams{Name: "Bad Name", InboundIDs: []int64{10}}, err: entity.ErrInvalidUserName,
		},
		{
			// Description is validated (length) before the registry is touched → no mocks.
			name: "error.description_too_long",
			in:   entity.UserCreateParams{Name: "postlog", Description: utils.Ptr(strings.Repeat("я", maxDescriptionLen+1)), InboundIDs: []int64{10}},
			err:  entity.ErrDescriptionTooLong,
		},
		{
			// Name uniqueness now comes from the users.name constraint: Create returns
			// entity.ErrNameTaken (no pre-check). The service propagates it.
			name: "error.name_taken", in: entity.UserCreateParams{Name: "postlog", InboundIDs: []int64{10}},
			err: entity.ErrNameTaken,
			buildMocks: func(m *mocks) {
				m.nodes.EXPECT().List(gomock.Any()).Return(n1(), nil)
				m.client.EXPECT().ListInbounds(gomock.Any(), n1Target()).Return(panelInbounds(), nil)
				m.users.EXPECT().Create(gomock.Any(), &entity.User{Name: "postlog", SubID: fixedSubID, Connections: []entity.Connection{{InboundID: 10}}}).Return(entity.ErrNameTaken)
			},
		},
		{
			name: "error.unknown_inbound", in: entity.UserCreateParams{Name: "postlog", InboundIDs: []int64{999}},
			err: entity.ErrInboundNotFound,
			buildMocks: func(m *mocks) {
				m.nodes.EXPECT().List(gomock.Any()).Return(n1(), nil)
			},
		},
		{
			// A non-positive inbound id (the moved-from-schema minimum:1 guard) has no
			// match — same as any unknown id → ErrInboundNotFound (no longer silently skipped).
			name: "error.zero_inbound", in: entity.UserCreateParams{Name: "postlog", InboundIDs: []int64{0}},
			err: entity.ErrInboundNotFound,
			buildMocks: func(m *mocks) {
				m.nodes.EXPECT().List(gomock.Any()).Return(n1(), nil)
			},
		},
		{
			name: "error.create_repo", in: entity.UserCreateParams{Name: "postlog", InboundIDs: []int64{10}},
			err: targetErr,
			buildMocks: func(m *mocks) {
				m.nodes.EXPECT().List(gomock.Any()).Return(n1(), nil)
				// pre-flight email check: panel is free
				m.client.EXPECT().ListInbounds(gomock.Any(), n1Target()).Return(panelInbounds(), nil)
				m.users.EXPECT().Create(gomock.Any(), &entity.User{Name: "postlog", SubID: fixedSubID, Connections: []entity.Connection{{InboundID: 10}}}).Return(targetErr)
			},
		},
		{
			// A client with our email (nickname) already exists on a target panel —
			// orphan, manual, or half-finished delete. Create must ABORT, touching
			// nothing: no store row, no DelClient, no AddClient.
			name: "error.email_exists_on_panel",
			in:   entity.UserCreateParams{Name: "postlog", InboundIDs: []int64{10}},
			err:  entity.PanelClientExistsError{Node: "N1"},
			buildMocks: func(m *mocks) {
				m.nodes.EXPECT().List(gomock.Any()).Return(n1(), nil)
				m.client.EXPECT().ListInbounds(gomock.Any(), n1Target()).Return([]entity.PanelInbound{
					{ID: 1, Port: 4433, Enable: true, Clients: []entity.PanelClient{{Email: "postlog", UUID: uuid.New()}}},
				}, nil)
				// no Create, no DelClient, no AddClient
			},
		},
		{
			// Description is trimmed before storage: the leading/trailing spaces are dropped,
			// so Create receives the normalised value.
			name:      "success.two_inbounds_same_node",
			in:        entity.UserCreateParams{Name: "postlog", Description: utils.Ptr("  рабочий ноутбук  "), InboundIDs: []int64{10, 11}},
			wantConns: 2,
			buildMocks: func(m *mocks) {
				m.nodes.EXPECT().List(gomock.Any()).Return(n1(), nil)
				m.users.EXPECT().Create(gomock.Any(), &entity.User{Name: "postlog", SubID: fixedSubID, Description: utils.Ptr("рабочий ноутбук"), Connections: []entity.Connection{{InboundID: 10}, {InboundID: 11}}}).Return(nil)
				// ListInbounds twice: pre-flight email check + syncPanels lookup; the
				// panel has no "postlog" client → free.
				m.client.EXPECT().ListInbounds(gomock.Any(), n1Target()).Return(panelInbounds(), nil).Times(2)
				// one panel, two inbounds (4433+8443) → single add with both 3x-ui ids.
				m.client.EXPECT().AddClient(gomock.Any(), n1Target(), []int{1, 2}, entity.ClientSpec{ID: fixedClientID, Email: "postlog", Flow: "", SubID: fixedSubID}).Return(nil)
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

			svc := newService(m)
			u, err := svc.CreateUser(ctx, tc.in)

			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, u.Connections, tc.wantConns)
			assert.Equal(t, fixedSubID, u.SubID)

			wantDesc, _ := validateDescription(tc.in.Description)
			assert.Equal(t, wantDesc, u.Description)
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
				m.client.EXPECT().DelClient(gomock.Any(), n1Target(), "postlog").Return(nil)
				m.users.EXPECT().Delete(gomock.Any(), int64(7)).Return(nil)
			},
		},
		{
			name: "success.node_absent_still_deletes_row",
			buildMocks: func(m *mocks) {
				m.users.EXPECT().Get(gomock.Any(), int64(7)).Return(user, nil)
				m.nodes.EXPECT().List(gomock.Any()).Return(nil, nil) // node gone from registry → skip del
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
			svc := newService(m)
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
