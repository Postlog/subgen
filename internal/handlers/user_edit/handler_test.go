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
)

func TestHandler_UserEdit(t *testing.T) {
	internalErr := errors.New("db down")

	tt := []struct {
		name string
		req  *oas.UserEditReq

		buildEditorMock func(m *Mockeditor)

		result oas.UserEditRes
		err    error
	}{
		{
			name: "success",
			req:  &oas.UserEditReq{ID: 7, InboundIDs: []int64{1, 2}},
			buildEditorMock: func(m *Mockeditor) {
				m.EXPECT().
					EditUser(gomock.Any(), int64(7), entity.ConnectionSelection{InboundIDs: []int64{1, 2}}).
					Return(nil)
			},
			result: &oas.MessageResponse{Message: "Подключения обновлены"},
		},
		{
			name: "error.no_connection",
			req:  &oas.UserEditReq{ID: 7, InboundIDs: []int64{}},
			buildEditorMock: func(m *Mockeditor) {
				m.EXPECT().EditUser(gomock.Any(), int64(7), gomock.Any()).Return(entity.ErrNoConnectionSelected)
			},
			result: &oas.UserEditBadRequest{ErrMessage: msgNoConnection},
		},
		{
			name: "error.inbound_not_found",
			req:  &oas.UserEditReq{ID: 7, InboundIDs: []int64{99}},
			buildEditorMock: func(m *Mockeditor) {
				m.EXPECT().EditUser(gomock.Any(), int64(7), gomock.Any()).Return(entity.ErrInboundNotFound)
			},
			result: &oas.UserEditBadRequest{ErrMessage: msgInboundNotFound},
		},
		{
			name: "error.internal",
			req:  &oas.UserEditReq{ID: 7, InboundIDs: []int64{1}},
			buildEditorMock: func(m *Mockeditor) {
				m.EXPECT().EditUser(gomock.Any(), int64(7), gomock.Any()).Return(internalErr)
			},
			err: internalErr,
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			svc := NewMockeditor(ctrl)
			if tc.buildEditorMock != nil {
				tc.buildEditorMock(svc)
			}

			res, err := New(svc).UserEdit(context.Background(), tc.req)

			require.ErrorIs(t, err, tc.err)
			assert.Equal(t, tc.result, res)
		})
	}
}
