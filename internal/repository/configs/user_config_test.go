//go:build integration

package configs_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/repository/configs"
	"github.com/postlog/subgen/internal/repository/dbtest"
	"github.com/postlog/subgen/internal/repository/routing"
)

// UserConfigID reports a user's custom config id, ok=false when they have none.
func TestRepository_UserConfigID(t *testing.T) {
	t.Parallel()

	t.Run("absent", func(t *testing.T) {
		t.Parallel()
		db := dbtest.OpenDB(t)
		repo := configs.New(db, routing.New(db))
		userID := seedUser(t, db, "alice", "sub-alice")

		_, ok, err := repo.UserConfigID(t.Context(), userID, entity.ConfigKindMihomo)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("present", func(t *testing.T) {
		t.Parallel()
		db := dbtest.OpenDB(t)
		repo := configs.New(db, routing.New(db))
		userID := seedUser(t, db, "bob", "sub-bob")

		want, err := repo.CreateUserConfig(t.Context(), userID, entity.ConfigKindMihomo)
		require.NoError(t, err)

		got, ok, err := repo.UserConfigID(t.Context(), userID, entity.ConfigKindMihomo)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, want, got)
	})
}
