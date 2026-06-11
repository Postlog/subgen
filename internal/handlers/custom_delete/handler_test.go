package custom_delete

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

func TestHandler_CustomDelete(t *testing.T) {
	internalErr := errors.New("db down")

	tt := []struct {
		name string
		req  *oas.CustomDeleteReq

		buildConfigsMock func(m *MockconfigDeleter)

		result oas.CustomDeleteRes
		err    error
	}{
		{
			name: "success",
			req:  &oas.CustomDeleteReq{UserId: 7},
			buildConfigsMock: func(m *MockconfigDeleter) {
				m.EXPECT().
					DeleteUserConfig(gomock.Any(), int64(7), entity.ConfigKindMihomo).
					Return(nil)
			},
			result: &oas.MessageResponse{Message: "Кастомный конфиг удалён"},
		},
		{
			name:   "error.invalid_id",
			req:    &oas.CustomDeleteReq{UserId: 0},
			result: &oas.CustomDeleteBadRequest{ErrMessage: msgInvalidID}, // guard, no service call
		},
		{
			name: "error.missing",
			req:  &oas.CustomDeleteReq{UserId: 7},
			buildConfigsMock: func(m *MockconfigDeleter) {
				m.EXPECT().
					DeleteUserConfig(gomock.Any(), int64(7), entity.ConfigKindMihomo).
					Return(entity.ErrUserConfigNotFound)
			},
			result: &oas.CustomDeleteBadRequest{ErrMessage: msgConfigMissing},
		},
		{
			name: "error.internal",
			req:  &oas.CustomDeleteReq{UserId: 7},
			buildConfigsMock: func(m *MockconfigDeleter) {
				m.EXPECT().
					DeleteUserConfig(gomock.Any(), int64(7), entity.ConfigKindMihomo).
					Return(internalErr)
			},
			err: internalErr,
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			configs := NewMockconfigDeleter(ctrl)
			if tc.buildConfigsMock != nil {
				tc.buildConfigsMock(configs)
			}

			res, err := New(configs).CustomDelete(context.Background(), tc.req)

			require.ErrorIs(t, err, tc.err)
			assert.Equal(t, tc.result, res)
		})
	}
}
