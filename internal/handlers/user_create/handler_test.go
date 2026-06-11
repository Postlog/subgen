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
	"github.com/postlog/subgen/internal/utils"
)

// params is the entity.UserCreateParams the handler builds from a request (no description).
func params(name string, ids ...int64) entity.UserCreateParams {
	return entity.UserCreateParams{Name: name, InboundIDs: ids}
}

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
			req:  &oas.UserCreateReq{Name: "alice", Description: oas.NewOptString("заметка"), InboundIDs: []int64{1, 2}},
			buildCreatorMock: func(m *Mockcreator) {
				m.EXPECT().CreateUser(gomock.Any(), entity.UserCreateParams{
					Name: "alice", Description: utils.Ptr("заметка"), InboundIDs: []int64{1, 2},
				}).Return(&entity.User{ID: 7}, nil)
			},
			result: &oas.MessageResponse{Message: MsgCreated},
		},
		{
			name: "error.name_taken",
			req:  &oas.UserCreateReq{Name: "bob", InboundIDs: []int64{1}},
			buildCreatorMock: func(m *Mockcreator) {
				m.EXPECT().CreateUser(gomock.Any(), params("bob", 1)).Return(nil, entity.ErrNameTaken)
			},
			result: &oas.UserCreateConflict{ErrMessage: MsgNameTaken},
		},
		{
			name: "error.panel_client_exists",
			req:  &oas.UserCreateReq{Name: "bob", InboundIDs: []int64{1}},
			buildCreatorMock: func(m *Mockcreator) {
				m.EXPECT().CreateUser(gomock.Any(), params("bob", 1)).Return(nil, entity.PanelClientExistsError{Node: "N1"})
			},
			result: &oas.UserCreateConflict{ErrMessage: "на панели «N1» уже есть клиент с таким именем — удалите его там вручную или выберите другое имя"},
		},
		{
			name: "error.invalid_name",
			req:  &oas.UserCreateReq{Name: "bad name", InboundIDs: []int64{1}},
			buildCreatorMock: func(m *Mockcreator) {
				m.EXPECT().CreateUser(gomock.Any(), params("bad name", 1)).Return(nil, entity.ErrInvalidUserName)
			},
			result: &oas.UserCreateBadRequest{ErrMessage: MsgInvalidName},
		},
		{
			name: "error.no_connection",
			req:  &oas.UserCreateReq{Name: "carol", InboundIDs: []int64{1}},
			buildCreatorMock: func(m *Mockcreator) {
				m.EXPECT().CreateUser(gomock.Any(), params("carol", 1)).Return(nil, entity.ErrNoConnectionSelected)
			},
			result: &oas.UserCreateBadRequest{ErrMessage: MsgNoConnection},
		},
		{
			name: "error.description_too_long",
			req:  &oas.UserCreateReq{Name: "carol", InboundIDs: []int64{1}},
			buildCreatorMock: func(m *Mockcreator) {
				m.EXPECT().CreateUser(gomock.Any(), params("carol", 1)).Return(nil, entity.ErrDescriptionTooLong)
			},
			result: &oas.UserCreateBadRequest{ErrMessage: MsgDescTooLong},
		},
		{
			name: "error.inbound_not_found",
			req:  &oas.UserCreateReq{Name: "carol", InboundIDs: []int64{99}},
			buildCreatorMock: func(m *Mockcreator) {
				m.EXPECT().CreateUser(gomock.Any(), params("carol", 99)).Return(nil, entity.ErrInboundNotFound)
			},
			result: &oas.UserCreateBadRequest{ErrMessage: MsgInboundNotFound},
		},
		{
			name: "error.node_not_found",
			req:  &oas.UserCreateReq{Name: "carol", InboundIDs: []int64{1}},
			buildCreatorMock: func(m *Mockcreator) {
				m.EXPECT().CreateUser(gomock.Any(), params("carol", 1)).Return(nil, entity.ErrNodeNotFound)
			},
			result: &oas.UserCreateBadRequest{ErrMessage: MsgNodeNotFound},
		},
		{
			name: "error.internal",
			req:  &oas.UserCreateReq{Name: "dave", InboundIDs: []int64{1}},
			buildCreatorMock: func(m *Mockcreator) {
				m.EXPECT().CreateUser(gomock.Any(), params("dave", 1)).Return(nil, internalErr)
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
