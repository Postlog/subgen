package custom_create

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

func TestHandler_CustomCreate(t *testing.T) {
	internalErr := errors.New("db down")

	tt := []struct {
		name string
		req  *oas.CustomCreateReq

		buildConfigsMock func(m *MockconfigsRepo)

		result oas.CustomCreateRes
		err    error
	}{
		{
			name: "success",
			req:  &oas.CustomCreateReq{UserId: 7},
			buildConfigsMock: func(m *MockconfigsRepo) {
				m.EXPECT().
					CreateUserConfig(gomock.Any(), int64(7), entity.ConfigKindMihomo).
					Return(int64(42), nil)
			},
			result: &oas.MessageResponse{Message: "Custom config created"},
		},
		{
			name: "error.exists",
			req:  &oas.CustomCreateReq{UserId: 7},
			buildConfigsMock: func(m *MockconfigsRepo) {
				m.EXPECT().
					CreateUserConfig(gomock.Any(), int64(7), entity.ConfigKindMihomo).
					Return(int64(0), entity.ErrUserConfigExists)
			},
			result: &oas.CustomCreateBadRequest{ErrMessage: msgConfigExists},
		},
		{
			name: "error.internal",
			req:  &oas.CustomCreateReq{UserId: 7},
			buildConfigsMock: func(m *MockconfigsRepo) {
				m.EXPECT().
					CreateUserConfig(gomock.Any(), int64(7), entity.ConfigKindMihomo).
					Return(int64(0), internalErr)
			},
			err: internalErr,
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			configs := NewMockconfigsRepo(ctrl)
			if tc.buildConfigsMock != nil {
				tc.buildConfigsMock(configs)
			}

			res, err := New(configs).CustomCreate(context.Background(), tc.req)

			require.ErrorIs(t, err, tc.err)
			assert.Equal(t, tc.result, res)
		})
	}
}
