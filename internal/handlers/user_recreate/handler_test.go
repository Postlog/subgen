package user_recreate

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestHandler_ServeHTTP(t *testing.T) {
	targetErr := errors.New("test")

	tt := []struct {
		name      string
		buildMock func(m *Mockrecreator)
		wantOK    bool
	}{
		{
			name: "success",
			buildMock: func(m *Mockrecreator) {
				m.EXPECT().RecreateUser(gomock.Any(), int64(7)).Return(nil)
			},
			wantOK: true,
		},
		{
			name: "error.downstream",
			buildMock: func(m *Mockrecreator) {
				m.EXPECT().RecreateUser(gomock.Any(), int64(7)).Return(targetErr)
			},
			wantOK: false,
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			m := NewMockrecreator(ctrl)
			tc.buildMock(m)

			r := httptest.NewRequest(http.MethodPost, "/admin/api/users/recreate", strings.NewReader(`{"id":7}`))

			rec := httptest.NewRecorder()

			New(m).ServeHTTP(rec, r)

			body := rec.Body.String()
			if tc.wantOK {
				assert.Contains(t, body, `"ok":true`)
			} else {
				assert.Contains(t, body, `"ok":false`)
			}
		})
	}
}
