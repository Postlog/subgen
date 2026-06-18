package provider_check

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/oas"
)

func TestHandler_ProviderCheck(t *testing.T) {
	const (
		url    = "http://host/file"
		format = "yaml"
	)

	tt := []struct {
		name string
		req  *oas.ProviderCheckReq

		buildCheckerMock func(m *MockrulesetChecker)

		result oas.ProviderCheckRes
	}{
		{
			name: "ok",
			req:  &oas.ProviderCheckReq{URL: url, Format: format},
			buildCheckerMock: func(m *MockrulesetChecker) {
				m.EXPECT().Check(gomock.Any(), url, format).
					Return(entity.RulesetCheckResult{Outcome: entity.RulesetCheckOK, Size: 1024})
			},
			result: &oas.MessageResponse{Message: "Available: format \"yaml\", 1.0 KB"},
		},
		{
			name: "http_error",
			req:  &oas.ProviderCheckReq{URL: url, Format: format},
			buildCheckerMock: func(m *MockrulesetChecker) {
				m.EXPECT().Check(gomock.Any(), url, format).
					Return(entity.RulesetCheckResult{Outcome: entity.RulesetCheckHTTPError, Status: 404})
			},
			result: &oas.ProviderCheckBadRequest{ErrMessage: "The server returned HTTP 404 — no file or no access"},
		},
		{
			name: "empty",
			req:  &oas.ProviderCheckReq{URL: url, Format: format},
			buildCheckerMock: func(m *MockrulesetChecker) {
				m.EXPECT().Check(gomock.Any(), url, format).
					Return(entity.RulesetCheckResult{Outcome: entity.RulesetCheckEmpty})
			},
			result: &oas.ProviderCheckBadRequest{ErrMessage: MsgEmpty},
		},
		{
			name: "format_mismatch",
			req:  &oas.ProviderCheckReq{URL: url, Format: format},
			buildCheckerMock: func(m *MockrulesetChecker) {
				m.EXPECT().Check(gomock.Any(), url, format).
					Return(entity.RulesetCheckResult{Outcome: entity.RulesetCheckFormatMismatch, Size: 512})
			},
			result: &oas.ProviderCheckBadRequest{ErrMessage: "Downloaded (512 B), but the content does not look like the \"yaml\" format"},
		},
		{
			name: "unreachable",
			req:  &oas.ProviderCheckReq{URL: url, Format: format},
			buildCheckerMock: func(m *MockrulesetChecker) {
				m.EXPECT().Check(gomock.Any(), url, format).
					Return(entity.RulesetCheckResult{Outcome: entity.RulesetCheckUnreachable})
			},
			result: &oas.ProviderCheckBadRequest{ErrMessage: MsgUnreachable},
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			checker := NewMockrulesetChecker(ctrl)
			if tc.buildCheckerMock != nil {
				tc.buildCheckerMock(checker)
			}

			res, err := New(checker).ProviderCheck(context.Background(), tc.req)

			require.NoError(t, err)
			assert.Equal(t, tc.result, res)
		})
	}
}
