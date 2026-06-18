package user_edit

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

// editParams is the entity.UserEditParams the handler builds from a request (no description).
func editParams(id int64, ids ...int64) entity.UserEditParams {
	return entity.UserEditParams{ID: id, InboundIDs: ids}
}

func TestHandler_UserEdit(t *testing.T) {
	internalErr := errors.New("db down")

	tt := []struct {
		name string
		req  *oas.UserEditReq

		buildEditorMock func(m *MockprovisioningService)

		result oas.UserEditRes
		err    error
	}{
		{
			name: "success",
			req:  &oas.UserEditReq{ID: 7, Description: oas.NewOptString("note"), InboundIDs: []int64{1, 2}},
			buildEditorMock: func(m *MockprovisioningService) {
				m.EXPECT().EditUser(gomock.Any(), entity.UserEditParams{
					ID: 7, Description: utils.Ptr("note"), InboundIDs: []int64{1, 2},
				}).Return(nil)
			},
			result: &oas.MessageResponse{Message: MsgUpdated},
		},
		{
			name: "error.no_connection",
			req:  &oas.UserEditReq{ID: 7, InboundIDs: []int64{}},
			buildEditorMock: func(m *MockprovisioningService) {
				m.EXPECT().EditUser(gomock.Any(), entity.UserEditParams{ID: 7, InboundIDs: []int64{}}).Return(entity.ErrNoConnectionSelected)
			},
			result: &oas.UserEditBadRequest{ErrMessage: MsgNoConnection},
		},
		{
			name: "error.inbound_not_found",
			req:  &oas.UserEditReq{ID: 7, InboundIDs: []int64{99}},
			buildEditorMock: func(m *MockprovisioningService) {
				m.EXPECT().EditUser(gomock.Any(), editParams(7, 99)).Return(entity.ErrInboundNotFound)
			},
			result: &oas.UserEditBadRequest{ErrMessage: MsgInboundNotFound},
		},
		{
			name: "error.description_too_long",
			req:  &oas.UserEditReq{ID: 7, InboundIDs: []int64{1}},
			buildEditorMock: func(m *MockprovisioningService) {
				m.EXPECT().EditUser(gomock.Any(), editParams(7, 1)).Return(entity.ErrDescriptionTooLong)
			},
			result: &oas.UserEditBadRequest{ErrMessage: MsgDescTooLong},
		},
		{
			name: "error.internal",
			req:  &oas.UserEditReq{ID: 7, InboundIDs: []int64{1}},
			buildEditorMock: func(m *MockprovisioningService) {
				m.EXPECT().EditUser(gomock.Any(), editParams(7, 1)).Return(internalErr)
			},
			err: internalErr,
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			svc := NewMockprovisioningService(ctrl)
			if tc.buildEditorMock != nil {
				tc.buildEditorMock(svc)
			}

			res, err := New(svc).UserEdit(context.Background(), tc.req)

			require.ErrorIs(t, err, tc.err)
			assert.Equal(t, tc.result, res)
		})
	}
}
