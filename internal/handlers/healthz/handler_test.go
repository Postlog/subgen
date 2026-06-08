package healthz

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler_Healthz(t *testing.T) {
	t.Parallel()

	res, err := New().Healthz(context.Background())
	require.NoError(t, err)

	body, err := io.ReadAll(res.Data)
	require.NoError(t, err)
	assert.Equal(t, "ok\n", string(body))
}
