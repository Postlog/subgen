package user_edit

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/postlog/subgen/internal/entity"
)

func TestHandler_ServeHTTP(t *testing.T) {
	targetErr := errors.New("test")

	tt := []struct {
		name      string
		buildMock func(m *Mockeditor)
		wantOK    bool
	}{
		{
			name: "success",
			buildMock: func(m *Mockeditor) {
				m.EXPECT().EditUser(gomock.Any(), int64(7),
					entity.ConnectionSelection{InboundIDs: []int64{10, 11}}).Return(nil)
			},
			wantOK: true,
		},
		{
			name: "error.downstream",
			buildMock: func(m *Mockeditor) {
				m.EXPECT().EditUser(gomock.Any(), int64(7),
					entity.ConnectionSelection{InboundIDs: []int64{10, 11}}).Return(targetErr)
			},
			wantOK: false,
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			m := NewMockeditor(ctrl)
			tc.buildMock(m)

			r := httptest.NewRequest(http.MethodPost, "/admin/api/users/edit", strings.NewReader(`{"id":7,"inboundIDs":[10,11]}`))

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
