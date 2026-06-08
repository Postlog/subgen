package config_customs

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

func TestHandler_ConfigCustoms(t *testing.T) {
	internalErr := errors.New("db down")

	tt := []struct {
		name string

		buildConfigsMock func(m *MockconfigLister)
		buildUsersMock   func(m *MockuserLister)

		result oas.ConfigCustomsRes
		err    error
	}{
		{
			name: "success",
			buildConfigsMock: func(m *MockconfigLister) {
				m.EXPECT().UserConfigUserIDs(gomock.Any(), entity.ConfigKindMihomo).Return([]int64{2}, nil)
			},
			buildUsersMock: func(m *MockuserLister) {
				m.EXPECT().ListNames(gomock.Any()).Return([]entity.User{
					{ID: 1, Name: "alice"},
					{ID: 2, Name: "bob"},
				}, nil)
			},
			result: &oas.ConfigCustomsOK{
				Customs: []oas.ConfigCustomsOKCustomsItem{{UserId: 2, Name: "bob"}},
				Users: []oas.ConfigCustomsOKUsersItem{
					{ID: 1, Name: "alice"},
					{ID: 2, Name: "bob"},
				},
			},
		},
		{
			name: "error.list_ids",
			buildConfigsMock: func(m *MockconfigLister) {
				m.EXPECT().UserConfigUserIDs(gomock.Any(), entity.ConfigKindMihomo).Return(nil, internalErr)
			},
			err: internalErr,
		},
		{
			name: "error.list_names",
			buildConfigsMock: func(m *MockconfigLister) {
				m.EXPECT().UserConfigUserIDs(gomock.Any(), entity.ConfigKindMihomo).Return([]int64{}, nil)
			},
			buildUsersMock: func(m *MockuserLister) {
				m.EXPECT().ListNames(gomock.Any()).Return(nil, internalErr)
			},
			err: internalErr,
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			configs := NewMockconfigLister(ctrl)
			if tc.buildConfigsMock != nil {
				tc.buildConfigsMock(configs)
			}

			users := NewMockuserLister(ctrl)
			if tc.buildUsersMock != nil {
				tc.buildUsersMock(users)
			}

			res, err := New(configs, users).ConfigCustoms(context.Background())

			require.ErrorIs(t, err, tc.err)
			assert.Equal(t, tc.result, res)
		})
	}
}
