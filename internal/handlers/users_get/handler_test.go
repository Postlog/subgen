package users_get

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

func TestHandler_UsersGet(t *testing.T) {
	const subID = "sub-1"

	internalErr := errors.New("db down")

	tt := []struct {
		name   string
		params oas.UsersGetParams

		buildUsersMock func(m *MockusersRepo)
		buildFleetMock func(m *MockfleetService)
		buildLinksMock func(m *MocksublinksService)

		result oas.UsersGetRes
		err    error
	}{
		{
			name:   "success",
			params: oas.UsersGetParams{},
			buildUsersMock: func(m *MockusersRepo) {
				m.EXPECT().
					ListPage(gomock.Any(), entity.UserListParams{Limit: 50, Offset: 0}).
					Return(entity.UserPage{
						Users: []entity.User{{
							ID:    7,
							Name:  "alice",
							SubID: subID,
							Connections: []entity.Connection{{
								InboundID: 3, Node: "RU1", Name: "force", Port: 8443,
							}},
						}},
						Total: 1,
					}, nil)
			},
			buildFleetMock: func(m *MockfleetService) {
				m.EXPECT().Fleet(gomock.Any()).Return(&entity.Fleet{}, nil)
			},
			buildLinksMock: func(m *MocksublinksService) {
				m.EXPECT().Links(gomock.Any(), gomock.Any()).Return(map[int64][]entity.SubLink{
					7: {
						{Title: "Mihomo", Value: "http://base/sub/mihomo/tok"},
						{Title: "Clashmi", Value: "clashmi://install-config?url=enc&name=Freedom&overwrite=false"},
					},
				}, nil)
			},
			result: &oas.UsersGetOK{
				Users: []oas.UsersGetOKUsersItem{{
					ID:   7,
					Name: "alice",
					Sub: oas.UsersGetOKUsersItemSub{
						Links: []oas.UsersGetOKUsersItemSubLinksItem{
							{Title: "Mihomo", Value: "http://base/sub/mihomo/tok"},
							{Title: "Clashmi", Value: "clashmi://install-config?url=enc&name=Freedom&overwrite=false"},
						},
					},
					Inbounds: []oas.UsersGetOKUsersItemInboundsItem{{
						ID: 3, Label: "RU1-force", Port: 8443, Missing: false,
					}},
				}},
				Total:   1,
				Page:    1,
				PerPage: 50,
			},
		},
		{
			name:   "empty",
			params: oas.UsersGetParams{},
			buildUsersMock: func(m *MockusersRepo) {
				m.EXPECT().
					ListPage(gomock.Any(), entity.UserListParams{Limit: 50, Offset: 0}).
					Return(entity.UserPage{}, nil)
			},
			buildFleetMock: func(m *MockfleetService) {
				m.EXPECT().Fleet(gomock.Any()).Return(&entity.Fleet{}, nil)
			},
			buildLinksMock: func(m *MocksublinksService) {
				m.EXPECT().Links(gomock.Any(), gomock.Any()).Return(map[int64][]entity.SubLink{}, nil)
			},
			result: &oas.UsersGetOK{
				Users:   []oas.UsersGetOKUsersItem{},
				Total:   0,
				Page:    1,
				PerPage: 50,
			},
		},
		{
			name:   "error.list",
			params: oas.UsersGetParams{},
			buildUsersMock: func(m *MockusersRepo) {
				m.EXPECT().ListPage(gomock.Any(), entity.UserListParams{Limit: 50, Offset: 0}).Return(entity.UserPage{}, internalErr)
			},
			err: internalErr,
		},
		{
			name:   "error.links",
			params: oas.UsersGetParams{},
			buildUsersMock: func(m *MockusersRepo) {
				m.EXPECT().
					ListPage(gomock.Any(), entity.UserListParams{Limit: 50, Offset: 0}).
					Return(entity.UserPage{Users: []entity.User{{ID: 7, Name: "alice", SubID: subID}}, Total: 1}, nil)
			},
			buildFleetMock: func(m *MockfleetService) {
				m.EXPECT().Fleet(gomock.Any()).Return(&entity.Fleet{}, nil)
			},
			buildLinksMock: func(m *MocksublinksService) {
				m.EXPECT().Links(gomock.Any(), gomock.Any()).Return(nil, internalErr)
			},
			err: internalErr,
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			users := NewMockusersRepo(ctrl)
			if tc.buildUsersMock != nil {
				tc.buildUsersMock(users)
			}

			fleet := NewMockfleetService(ctrl)
			if tc.buildFleetMock != nil {
				tc.buildFleetMock(fleet)
			}

			links := NewMocksublinksService(ctrl)
			if tc.buildLinksMock != nil {
				tc.buildLinksMock(links)
			}

			res, err := New(users, fleet, links).UsersGet(context.Background(), tc.params)

			require.ErrorIs(t, err, tc.err)
			assert.Equal(t, tc.result, res)
		})
	}
}
