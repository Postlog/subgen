package user_delete

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
		buildMock func(m *Mockdeleter)
		wantOK    bool
	}{
		{
			name: "success",
			buildMock: func(m *Mockdeleter) {
				m.EXPECT().DeleteUser(gomock.Any(), int64(7)).Return(nil)
			},
			wantOK: true,
		},
		{
			name: "error.downstream",
			buildMock: func(m *Mockdeleter) {
				m.EXPECT().DeleteUser(gomock.Any(), int64(7)).Return(targetErr)
			},
			wantOK: false,
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			m := NewMockdeleter(ctrl)
			tc.buildMock(m)

			r := httptest.NewRequest(http.MethodPost, "/admin/api/users/delete", strings.NewReader(`{"id":7}`))

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
