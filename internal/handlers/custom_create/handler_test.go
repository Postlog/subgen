package custom_create

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/postlog/subgen/internal/entity"
)

func TestHandler_ServeHTTP(t *testing.T) {
	tt := []struct {
		name      string
		body      string
		buildMock func(m *MockconfigCreator)
		wantOK    bool
	}{
		{name: "error.bad_json", body: `{bad`},
		{name: "error.no_user_id", body: `{"userId":0}`},
		{
			name: "error.already_exists",
			body: `{"userId":7}`,
			buildMock: func(m *MockconfigCreator) {
				m.EXPECT().CreateUserConfig(gomock.Any(), int64(7), entity.ConfigKindMihomo).
					Return(int64(0), entity.ErrUserConfigExists)
			},
		},
		{
			name:   "success",
			body:   `{"userId":7}`,
			wantOK: true,
			buildMock: func(m *MockconfigCreator) {
				m.EXPECT().CreateUserConfig(gomock.Any(), int64(7), entity.ConfigKindMihomo).
					Return(int64(3), nil)
			},
		},
	}

	t.Parallel()
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			mock := NewMockconfigCreator(ctrl)
			if tc.buildMock != nil {
				tc.buildMock(mock)
			}

			h := New(mock)
			req := httptest.NewRequest(http.MethodPost, "/admin/api/config/mihomo/custom/create", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			var body struct {
				OK bool `json:"ok"`
			}

			require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
			assert.Equal(t, tc.wantOK, body.OK)
		})
	}
}
