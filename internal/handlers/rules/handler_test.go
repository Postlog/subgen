package rules

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/postlog/subgen/internal/oas"
)

func TestHandler_Rules(t *testing.T) {
	data := []byte("payload: yes\n")

	tt := []struct {
		name   string
		params oas.RulesParams

		buildMirrorMock func(m *MockruleFiles)

		assertFn func(t *testing.T, res oas.RulesRes)
	}{
		{
			name:   "success",
			params: oas.RulesParams{File: "geosite.yaml"},
			buildMirrorMock: func(m *MockruleFiles) {
				m.EXPECT().Get("geosite.yaml").Return(data, "text/yaml", true)
			},
			assertFn: func(t *testing.T, res oas.RulesRes) {
				ok, isType := res.(*oas.RulesOKHeaders)
				require.True(t, isType, "expected *oas.RulesOKHeaders, got %T", res)
				assert.Equal(t, oas.NewOptString("nosniff"), ok.XContentTypeOptions)
				got, err := io.ReadAll(ok.Response.Data)
				require.NoError(t, err)
				assert.Equal(t, data, got)
			},
		},
		{
			name:   "notfound.unknown_file",
			params: oas.RulesParams{File: "missing.yaml"},
			buildMirrorMock: func(m *MockruleFiles) {
				m.EXPECT().Get("missing.yaml").Return(nil, "", false)
			},
			assertFn: func(t *testing.T, res oas.RulesRes) {
				_, isType := res.(*oas.RulesNotFound)
				assert.True(t, isType, "expected *oas.RulesNotFound, got %T", res)
			},
		},
		{
			name:   "notfound.empty_file",
			params: oas.RulesParams{File: ""},
			assertFn: func(t *testing.T, res oas.RulesRes) {
				_, isType := res.(*oas.RulesNotFound)
				assert.True(t, isType, "expected *oas.RulesNotFound, got %T", res)
			},
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			mirror := NewMockruleFiles(ctrl)
			if tc.buildMirrorMock != nil {
				tc.buildMirrorMock(mirror)
			}

			res, err := New(mirror).Rules(context.Background(), tc.params)

			require.NoError(t, err)
			tc.assertFn(t, res)
		})
	}
}
