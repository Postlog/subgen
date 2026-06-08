package logout

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/handlers/web"
)

func TestHandler_Logout(t *testing.T) {
	t.Parallel()

	res, err := New(web.NewSession("hmac-secret")).Logout(context.Background())
	require.NoError(t, err)

	require.NotNil(t, res)
	assert.True(t, res.SetCookie.IsSet())
}
