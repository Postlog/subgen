//go:build integration

package configs_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/mihomo"
	"github.com/postlog/subgen/internal/repository/configs"
	"github.com/postlog/subgen/internal/repository/dbtest"
	"github.com/postlog/subgen/internal/repository/routing"
)

// DeleteUserConfig removes the user's custom config (content cascades), and reports
// ErrUserConfigNotFound when there is nothing to delete.
func TestRepository_DeleteUserConfig(t *testing.T) {
	t.Parallel()

	t.Run("success.removes_and_cascades", func(t *testing.T) {
		t.Parallel()
		db := dbtest.OpenDB(t)
		rt := routing.New(db)
		repo := configs.New(db, rt)
		userID := seedUser(t, db, "alice", "sub-alice")

		id, err := repo.CreateUserConfig(t.Context(), userID, entity.ConfigKindMihomo)
		require.NoError(t, err)
		require.NoError(t, rt.SaveMihomoConfig(t.Context(), id, nil, nil,
			[]mihomo.RuleProvider{{Name: "p", Behavior: "domain", Format: "yaml", URL: "http://p"}}, "x: 1", mihomo.Profile{}))

		require.NoError(t, repo.DeleteUserConfig(t.Context(), userID, entity.ConfigKindMihomo))

		// Gone from the anchor and from content (cascade).
		_, ok, err := repo.UserConfigID(t.Context(), userID, entity.ConfigKindMihomo)
		require.NoError(t, err)
		assert.False(t, ok)

		provs, err := rt.RuleProviders(t.Context(), id)
		require.NoError(t, err)
		assert.Empty(t, provs)
	})

	t.Run("error.not_found", func(t *testing.T) {
		t.Parallel()
		db := dbtest.OpenDB(t)
		repo := configs.New(db, routing.New(db))
		userID := seedUser(t, db, "bob", "sub-bob")

		err := repo.DeleteUserConfig(t.Context(), userID, entity.ConfigKindMihomo)
		require.ErrorIs(t, err, entity.ErrUserConfigNotFound)
	})
}

// UserConfigUserIDs lists the users that have a custom config for the engine.
func TestRepository_UserConfigUserIDs(t *testing.T) {
	t.Parallel()

	db := dbtest.OpenDB(t)
	repo := configs.New(db, routing.New(db))

	a := seedUser(t, db, "alice", "sub-alice")
	_ = seedUser(t, db, "bob", "sub-bob") // no custom config
	c := seedUser(t, db, "carol", "sub-carol")

	_, err := repo.CreateUserConfig(t.Context(), a, entity.ConfigKindMihomo)
	require.NoError(t, err)
	_, err = repo.CreateUserConfig(t.Context(), c, entity.ConfigKindMihomo)
	require.NoError(t, err)

	got, err := repo.UserConfigUserIDs(t.Context(), entity.ConfigKindMihomo)
	require.NoError(t, err)
	assert.Equal(t, []int64{a, c}, got) // ordered by user_id, bob excluded
}
