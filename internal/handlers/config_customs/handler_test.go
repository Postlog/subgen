package config_customs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/postlog/subgen/internal/entity"
)

func TestHandler_ServeHTTP(t *testing.T) {
	tt := []struct {
		name       string
		buildMocks func(c *MockconfigLister, u *MockuserLister)
		want       []customView
		wantUsers  []userView
	}{
		{
			name: "empty",
			buildMocks: func(c *MockconfigLister, u *MockuserLister) {
				c.EXPECT().UserConfigUserIDs(gomock.Any(), entity.ConfigKindMihomo).Return(nil, nil)
				u.EXPECT().ListNames(gomock.Any()).Return(nil, nil)
			},
			want:      []customView{},
			wantUsers: []userView{},
		},
		{
			name: "success.sorted_by_name",
			buildMocks: func(c *MockconfigLister, u *MockuserLister) {
				c.EXPECT().UserConfigUserIDs(gomock.Any(), entity.ConfigKindMihomo).Return([]int64{1, 2}, nil)
				u.EXPECT().ListNames(gomock.Any()).Return([]entity.User{
					{ID: 1, Name: "zoe"}, {ID: 2, Name: "amy"}, {ID: 3, Name: "ben"},
				}, nil)
			},
			want: []customView{{UserID: 2, Name: "amy"}, {UserID: 1, Name: "zoe"}},
			// users mirrors the (already name-ordered) ListNames result, all of them.
			wantUsers: []userView{{ID: 1, Name: "zoe"}, {ID: 2, Name: "amy"}, {ID: 3, Name: "ben"}},
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			cfgs := NewMockconfigLister(ctrl)
			usrs := NewMockuserLister(ctrl)

			if tc.buildMocks != nil {
				tc.buildMocks(cfgs, usrs)
			}

			h := New(cfgs, usrs)
			req := httptest.NewRequest(http.MethodGet, "/admin/api/config/mihomo/customs", nil)
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			var body struct {
				Customs []customView `json:"customs"`
				Users   []userView   `json:"users"`
			}

			require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
			assert.Equal(t, tc.want, body.Customs)
			assert.Equal(t, tc.wantUsers, body.Users)
		})
	}
}
