package admin_shell

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/handlers/web"
	"github.com/postlog/subgen/internal/oas"
)

func TestHandler_AdminShell(t *testing.T) {
	sess := web.NewSession("hmac-secret")
	validCookie := sess.IssueCookie().Value

	tt := []struct {
		name   string
		cookie oas.OptString

		assert func(t *testing.T, res oas.AdminShellRes)
	}{
		{
			name:   "authed.serves_shell",
			cookie: oas.NewOptString(validCookie),
			assert: func(t *testing.T, res oas.AdminShellRes) {
				ok, isOK := res.(*oas.AdminShellOK)
				require.True(t, isOK, "want *oas.AdminShellOK, got %T", res)

				b, err := io.ReadAll(ok.Data)
				require.NoError(t, err)
				assert.NotEmpty(t, b, "the shell must have a body")
			},
		},
		{
			name:   "unauthed.redirects_to_login",
			cookie: oas.OptString{},
			assert: func(t *testing.T, res oas.AdminShellRes) {
				assert.Equal(t, &oas.AdminShellFound{Location: oas.NewOptString("/admin/login")}, res)
			},
		},
		{
			name:   "bad_cookie.redirects_to_login",
			cookie: oas.NewOptString("garbage"),
			assert: func(t *testing.T, res oas.AdminShellRes) {
				assert.Equal(t, &oas.AdminShellFound{Location: oas.NewOptString("/admin/login")}, res)
			},
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			res, err := New(sess, "").AdminShell(context.Background(), oas.AdminShellParams{SubgenAdmin: tc.cookie})

			require.NoError(t, err)
			tc.assert(t, res)
		})
	}
}

func TestHandler_AdminShellView(t *testing.T) {
	sess := web.NewSession("hmac-secret")
	validCookie := sess.IssueCookie().Value

	tt := []struct {
		name   string
		cookie oas.OptString

		assert func(t *testing.T, res oas.AdminShellViewRes)
	}{
		{
			name:   "authed.serves_shell",
			cookie: oas.NewOptString(validCookie),
			assert: func(t *testing.T, res oas.AdminShellViewRes) {
				ok, isOK := res.(*oas.AdminShellViewOK)
				require.True(t, isOK, "want *oas.AdminShellViewOK, got %T", res)

				b, err := io.ReadAll(ok.Data)
				require.NoError(t, err)
				assert.NotEmpty(t, b, "the shell must have a body")
			},
		},
		{
			name:   "unauthed.redirects_to_login",
			cookie: oas.OptString{},
			assert: func(t *testing.T, res oas.AdminShellViewRes) {
				assert.Equal(t, &oas.AdminShellViewFound{Location: oas.NewOptString("/admin/login")}, res)
			},
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			res, err := New(sess, "").AdminShellView(context.Background(), oas.AdminShellViewParams{View: "users", SubgenAdmin: tc.cookie})

			require.NoError(t, err)
			tc.assert(t, res)
		})
	}
}
