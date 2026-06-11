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

		buildCheckerMock func(m *MockproviderChecker)

		result oas.ProviderCheckRes
	}{
		{
			name: "ok",
			req:  &oas.ProviderCheckReq{URL: url, Format: format},
			buildCheckerMock: func(m *MockproviderChecker) {
				m.EXPECT().Check(gomock.Any(), url, format).
					Return(entity.RulesetCheckResult{Outcome: entity.RulesetCheckOK, Size: 1024})
			},
			result: &oas.MessageResponse{Message: "Доступен:  формат «yaml», 1.0 KB"},
		},
		{
			name: "http_error",
			req:  &oas.ProviderCheckReq{URL: url, Format: format},
			buildCheckerMock: func(m *MockproviderChecker) {
				m.EXPECT().Check(gomock.Any(), url, format).
					Return(entity.RulesetCheckResult{Outcome: entity.RulesetCheckHTTPError, Status: 404})
			},
			result: &oas.ProviderCheckBadRequest{ErrMessage: "Сервер вернул HTTP 404 — файла нет или нет доступа"},
		},
		{
			name: "empty",
			req:  &oas.ProviderCheckReq{URL: url, Format: format},
			buildCheckerMock: func(m *MockproviderChecker) {
				m.EXPECT().Check(gomock.Any(), url, format).
					Return(entity.RulesetCheckResult{Outcome: entity.RulesetCheckEmpty})
			},
			result: &oas.ProviderCheckBadRequest{ErrMessage: MsgEmpty},
		},
		{
			name: "format_mismatch",
			req:  &oas.ProviderCheckReq{URL: url, Format: format},
			buildCheckerMock: func(m *MockproviderChecker) {
				m.EXPECT().Check(gomock.Any(), url, format).
					Return(entity.RulesetCheckResult{Outcome: entity.RulesetCheckFormatMismatch, Size: 512})
			},
			result: &oas.ProviderCheckBadRequest{ErrMessage: "Скачалось (512 B), но содержимое не похоже на формат «yaml»"},
		},
		{
			name: "unreachable",
			req:  &oas.ProviderCheckReq{URL: url, Format: format},
			buildCheckerMock: func(m *MockproviderChecker) {
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

			checker := NewMockproviderChecker(ctrl)
			if tc.buildCheckerMock != nil {
				tc.buildCheckerMock(checker)
			}

			res, err := New(checker).ProviderCheck(context.Background(), tc.req)

			require.NoError(t, err)
			assert.Equal(t, tc.result, res)
		})
	}
}
