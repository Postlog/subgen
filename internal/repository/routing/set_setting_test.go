//go:build integration

package routing_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/repository/dbtest"
	"github.com/postlog/subgen/internal/repository/routing"
)

// TestRepository_SetSetting covers the upsert: the first call inserts the key, the
// second with the same key overwrites the value in place (ON CONFLICT). Read back via
// Setting. Straight-line, no table.
func TestRepository_SetSetting(t *testing.T) {
	t.Parallel()
	repo := routing.New(dbtest.OpenDB(t))

	require.NoError(t, repo.SetSetting(t.Context(), "base_yaml", "first"))
	got, err := repo.Setting(t.Context(), "base_yaml")
	require.NoError(t, err)
	assert.Equal(t, "first", got)

	// Same key → value overwritten, not duplicated.
	require.NoError(t, repo.SetSetting(t.Context(), "base_yaml", "second"))
	got, err = repo.Setting(t.Context(), "base_yaml")
	require.NoError(t, err)
	assert.Equal(t, "second", got)
}
