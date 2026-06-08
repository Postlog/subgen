package login

import (
	"context"
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
				assert.Equal(t, &oas.ErrorResponse{ErrMessage: "Неверный логин или пароль"}, res)
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
