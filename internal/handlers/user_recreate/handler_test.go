package user_recreate

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/postlog/subgen/internal/oas"
)

func TestHandler_UserRecreate(t *testing.T) {
	internalErr := errors.New("db down")

	tt := []struct {
		name string
		req  *oas.UserRecreateReq

		buildRecreatorMock func(m *Mockrecreator)

		result oas.UserRecreateRes
		err    error
	}{
		{
			name: "success",
			req:  &oas.UserRecreateReq{ID: 7},
			buildRecreatorMock: func(m *Mockrecreator) {
				m.EXPECT().RecreateUser(gomock.Any(), int64(7)).Return(nil)
			},
			result: &oas.MessageResponse{Message: MsgRecreated},
		},
		{
			name: "error.internal",
			req:  &oas.UserRecreateReq{ID: 7},
			buildRecreatorMock: func(m *Mockrecreator) {
				m.EXPECT().RecreateUser(gomock.Any(), int64(7)).Return(internalErr)
			},
			err: internalErr,
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			svc := NewMockrecreator(ctrl)
			if tc.buildRecreatorMock != nil {
				tc.buildRecreatorMock(svc)
			}

			res, err := New(svc).UserRecreate(context.Background(), tc.req)

			require.ErrorIs(t, err, tc.err)
			assert.Equal(t, tc.result, res)
		})
	}
}
