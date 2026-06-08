package user_create

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

func TestHandler_UserCreate(t *testing.T) {
	internalErr := errors.New("db down")

	tt := []struct {
		name string
		req  *oas.UserCreateReq

		buildCreatorMock func(m *Mockcreator)

		result oas.UserCreateRes
		err    error
	}{
		{
			name: "success",
			req:  &oas.UserCreateReq{Name: "alice", InboundIDs: []int64{1, 2}},
			buildCreatorMock: func(m *Mockcreator) {
				m.EXPECT().
					CreateUser(gomock.Any(), "alice", entity.ConnectionSelection{InboundIDs: []int64{1, 2}}).
					Return(&entity.User{ID: 7}, nil)
			},
			result: &oas.MessageResponse{Message: "Создан пользователь"},
		},
		{
			name: "error.name_taken",
			req:  &oas.UserCreateReq{Name: "bob", InboundIDs: []int64{1}},
			buildCreatorMock: func(m *Mockcreator) {
				m.EXPECT().CreateUser(gomock.Any(), "bob", gomock.Any()).Return(nil, entity.ErrNameTaken)
			},
			result: &oas.UserCreateConflict{ErrMessage: msgNameTaken},
		},
		{
			name: "error.panel_client_exists",
			req:  &oas.UserCreateReq{Name: "bob", InboundIDs: []int64{1}},
			buildCreatorMock: func(m *Mockcreator) {
				m.EXPECT().CreateUser(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, entity.PanelClientExistsError{Node: "N1"})
			},
			result: &oas.UserCreateConflict{ErrMessage: "на панели «N1» уже есть клиент с таким именем — удалите его там вручную или выберите другое имя"},
		},
		{
			name: "error.invalid_name",
			req:  &oas.UserCreateReq{Name: "bad name", InboundIDs: []int64{1}},
			buildCreatorMock: func(m *Mockcreator) {
				m.EXPECT().CreateUser(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, entity.ErrInvalidUserName)
			},
			result: &oas.UserCreateBadRequest{ErrMessage: msgInvalidName},
		},
		{
			name: "error.no_connection",
			req:  &oas.UserCreateReq{Name: "carol", InboundIDs: []int64{1}},
			buildCreatorMock: func(m *Mockcreator) {
				m.EXPECT().CreateUser(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, entity.ErrNoConnectionSelected)
			},
			result: &oas.UserCreateBadRequest{ErrMessage: msgNoConnection},
		},
		{
			name: "error.inbound_not_found",
			req:  &oas.UserCreateReq{Name: "carol", InboundIDs: []int64{99}},
			buildCreatorMock: func(m *Mockcreator) {
				m.EXPECT().CreateUser(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, entity.ErrInboundNotFound)
			},
			result: &oas.UserCreateBadRequest{ErrMessage: msgInboundNotFound},
		},
		{
			name: "error.node_not_found",
			req:  &oas.UserCreateReq{Name: "carol", InboundIDs: []int64{1}},
			buildCreatorMock: func(m *Mockcreator) {
				m.EXPECT().CreateUser(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, entity.ErrNodeNotFound)
			},
			result: &oas.UserCreateBadRequest{ErrMessage: msgNodeNotFound},
		},
		{
			name: "error.internal",
			req:  &oas.UserCreateReq{Name: "dave", InboundIDs: []int64{1}},
			buildCreatorMock: func(m *Mockcreator) {
				m.EXPECT().CreateUser(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, internalErr)
			},
			err: internalErr,
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			svc := NewMockcreator(ctrl)
			if tc.buildCreatorMock != nil {
				tc.buildCreatorMock(svc)
			}

			res, err := New(svc).UserCreate(context.Background(), tc.req)

			require.ErrorIs(t, err, tc.err)
			assert.Equal(t, tc.result, res)
		})
	}
}
