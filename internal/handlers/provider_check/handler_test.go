package provider_check

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/postlog/subgen/internal/entity"
)

func TestHandler_ServeHTTP(t *testing.T) {
	tt := []struct {
		name         string
		body         string
		buildMock    func(m *MockproviderChecker)
		wantOK       bool
		wantContains string
	}{
		{name: "error.bad_json", body: "{", wantContains: "JSON"},
		{name: "error.no_url", body: `{"url":"","format":"mrs"}`, wantContains: "Укажите URL"},
		{
			name: "success.ok", body: `{"url":"https://x","format":"mrs"}`, wantOK: true, wantContains: "Доступен",
			buildMock: func(m *MockproviderChecker) {
				m.EXPECT().Check(gomock.Any(), "https://x", "mrs").Return(entity.RulesetCheckResult{Outcome: entity.RulesetCheckOK, Size: 2048})
			},
		},
		{
			name: "error.http", body: `{"url":"https://x","format":"text"}`, wantContains: "HTTP 404",
			buildMock: func(m *MockproviderChecker) {
				m.EXPECT().Check(gomock.Any(), gomock.Any(), gomock.Any()).Return(entity.RulesetCheckResult{Outcome: entity.RulesetCheckHTTPError, Status: 404})
			},
		},
		{
			name: "error.format", body: `{"url":"https://x","format":"mrs"}`, wantContains: "не похоже",
			buildMock: func(m *MockproviderChecker) {
				m.EXPECT().Check(gomock.Any(), gomock.Any(), gomock.Any()).Return(entity.RulesetCheckResult{Outcome: entity.RulesetCheckFormatMismatch, Size: 50})
			},
		},
		{
			name: "error.unreachable", body: `{"url":"https://x","format":"text"}`, wantContains: "подключиться",
			buildMock: func(m *MockproviderChecker) {
				m.EXPECT().Check(gomock.Any(), gomock.Any(), gomock.Any()).Return(entity.RulesetCheckResult{Outcome: entity.RulesetCheckUnreachable, Detail: "no such host"})
			},
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			m := NewMockproviderChecker(ctrl)
			if tc.buildMock != nil {
				tc.buildMock(m)
			}

			rr := httptest.NewRecorder()
			New(m).ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(tc.body)))

			body := rr.Body.String()
			if tc.wantOK {
				assert.Contains(t, body, `"ok":true`)
			} else {
				assert.Contains(t, body, `"ok":false`)
			}

			assert.Contains(t, body, tc.wantContains)
		})
	}
}
