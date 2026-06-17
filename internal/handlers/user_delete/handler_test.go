package user_delete

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/postlog/subgen/internal/oas"
)

func TestHandler_UserDelete(t *testing.T) {
	internalErr := errors.New("db down")

	tt := []struct {
		name string
		req  *oas.UserDeleteReq

		buildDeleterMock func(m *MockprovisioningService)

		result oas.UserDeleteRes
		err    error
	}{
		{
			name: "success",
			req:  &oas.UserDeleteReq{ID: 7},
			buildDeleterMock: func(m *MockprovisioningService) {
				m.EXPECT().DeleteUser(gomock.Any(), int64(7)).Return(nil)
			},
			result: &oas.MessageResponse{Message: MsgDeleted},
		},
		{
			name: "error.internal",
			req:  &oas.UserDeleteReq{ID: 7},
			buildDeleterMock: func(m *MockprovisioningService) {
				m.EXPECT().DeleteUser(gomock.Any(), int64(7)).Return(internalErr)
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
			if tc.buildDeleterMock != nil {
				tc.buildDeleterMock(svc)
			}

			res, err := New(svc).UserDelete(context.Background(), tc.req)

			require.ErrorIs(t, err, tc.err)
			assert.Equal(t, tc.result, res)
		})
	}
}
