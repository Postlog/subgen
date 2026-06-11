package login

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/handlers/web"
	"github.com/postlog/subgen/internal/oas"
)

func TestHandler_Login(t *testing.T) {
	tt := []struct {
		name string
		req  *oas.LoginReq

		assert func(t *testing.T, res oas.LoginRes)
	}{
		{
			name: "success",
			req:  &oas.LoginReq{User: "admin", Password: "secret"},
			assert: func(t *testing.T, res oas.LoginRes) {
				ok, isOK := res.(*oas.MessageResponseHeaders)
				require.True(t, isOK, "want *oas.MessageResponseHeaders, got %T", res)
				assert.True(t, ok.SetCookie.IsSet())
				assert.Equal(t, "ok", ok.Response.Message)
			},
		},
		{
			name: "bad_creds",
			req:  &oas.LoginReq{User: "admin", Password: "wrong"},
			assert: func(t *testing.T, res oas.LoginRes) {
				assert.Equal(t, &oas.ErrorResponse{ErrMessage: MsgBadCredentials}, res)
			},
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sess := web.NewSession("hmac-secret")

			res, err := New(sess, "admin", "secret", "").Login(context.Background(), tc.req)

			require.NoError(t, err)
			tc.assert(t, res)
		})
	}
}

func TestHandler_LoginPage(t *testing.T) {
	sess := web.NewSession("hmac-secret")
	validCookie := sess.IssueCookie().Value

	tt := []struct {
		name   string
		cookie oas.OptString

		assert func(t *testing.T, res oas.LoginPageRes)
	}{
		{
			name:   "unauthed.serves_page",
			cookie: oas.OptString{},
			assert: func(t *testing.T, res oas.LoginPageRes) {
				ok, isOK := res.(*oas.LoginPageOK)
				require.True(t, isOK, "want *oas.LoginPageOK, got %T", res)

				b, err := io.ReadAll(ok.Data)
				require.NoError(t, err)
				assert.NotEmpty(t, b, "the login page must have a body")
			},
		},
		{
			name:   "authed.redirects_to_app",
			cookie: oas.NewOptString(validCookie),
			assert: func(t *testing.T, res oas.LoginPageRes) {
				assert.Equal(t, &oas.LoginPageFound{Location: oas.NewOptString("/admin/users")}, res)
			},
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			res, err := New(sess, "admin", "secret", "").LoginPage(context.Background(), oas.LoginPageParams{SubgenAdmin: tc.cookie})

			require.NoError(t, err)
			tc.assert(t, res)
		})
	}
}
