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
	tt := []struct {
		name string
		req  *oas.ProviderCheckReq

		buildCheckerMock func(m *MockproviderChecker)

		result oas.ProviderCheckRes
	}{
		{
			name: "ok",
			req:  &oas.ProviderCheckReq{URL: "http://host/file", Format: "yaml"},
			buildCheckerMock: func(m *MockproviderChecker) {
				m.EXPECT().Check(gomock.Any(), "http://host/file", "yaml").
					Return(entity.RulesetCheckResult{Outcome: entity.RulesetCheckOK, Size: 1024})
			},
			result: &oas.MessageResponse{Message: "Доступен:  формат «yaml», 1.0 KB"},
		},
		{
			name:   "empty_url",
			req:    &oas.ProviderCheckReq{URL: "   ", Format: "yaml"},
			result: &oas.ProviderCheckBadRequest{ErrMessage: msgURLRequired}, // guard, no checker call
		},
		{
			name: "http_error",
			req:  &oas.ProviderCheckReq{URL: "http://host/file", Format: "yaml"},
			buildCheckerMock: func(m *MockproviderChecker) {
				m.EXPECT().Check(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(entity.RulesetCheckResult{Outcome: entity.RulesetCheckHTTPError, Status: 404})
			},
			result: &oas.ProviderCheckBadRequest{ErrMessage: "Сервер вернул HTTP 404 — файла нет или нет доступа"},
		},
		{
			name: "empty",
			req:  &oas.ProviderCheckReq{URL: "http://host/file", Format: "yaml"},
			buildCheckerMock: func(m *MockproviderChecker) {
				m.EXPECT().Check(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(entity.RulesetCheckResult{Outcome: entity.RulesetCheckEmpty})
			},
			result: &oas.ProviderCheckBadRequest{ErrMessage: msgEmpty},
		},
		{
			name: "format_mismatch",
			req:  &oas.ProviderCheckReq{URL: "http://host/file", Format: "yaml"},
			buildCheckerMock: func(m *MockproviderChecker) {
				m.EXPECT().Check(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(entity.RulesetCheckResult{Outcome: entity.RulesetCheckFormatMismatch, Size: 512})
			},
			result: &oas.ProviderCheckBadRequest{ErrMessage: "Скачалось (512 B), но содержимое не похоже на формат «yaml»"},
		},
		{
			name: "unreachable",
			req:  &oas.ProviderCheckReq{URL: "http://host/file", Format: "yaml"},
			buildCheckerMock: func(m *MockproviderChecker) {
				m.EXPECT().Check(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(entity.RulesetCheckResult{Outcome: entity.RulesetCheckUnreachable})
			},
			result: &oas.ProviderCheckBadRequest{ErrMessage: msgUnreachable},
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
