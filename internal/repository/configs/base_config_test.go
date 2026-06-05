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

// BaseConfigID reports the engine's base config id, ok=false before it is created.
func TestRepository_BaseConfigID(t *testing.T) {
	t.Parallel()

	t.Run("absent", func(t *testing.T) {
		t.Parallel()
		repo := configs.New(dbtest.OpenDB(t), routing.New(dbtest.OpenDB(t)))

		_, ok, err := repo.BaseConfigID(t.Context(), entity.ConfigKindMihomo)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("present_after_ensure", func(t *testing.T) {
		t.Parallel()
		db := dbtest.OpenDB(t)
		repo := configs.New(db, routing.New(db))

		want, err := repo.EnsureBaseConfigID(t.Context(), entity.ConfigKindMihomo)
		require.NoError(t, err)

		got, ok, err := repo.BaseConfigID(t.Context(), entity.ConfigKindMihomo)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, want, got)
	})
}

// EnsureBaseConfigID creates the base row once and returns the same id on repeat.
func TestRepository_EnsureBaseConfigID(t *testing.T) {
	t.Parallel()

	db := dbtest.OpenDB(t)
	repo := configs.New(db, routing.New(db))

	first, err := repo.EnsureBaseConfigID(t.Context(), entity.ConfigKindMihomo)
	require.NoError(t, err)

	second, err := repo.EnsureBaseConfigID(t.Context(), entity.ConfigKindMihomo)
	require.NoError(t, err)

	assert.Equal(t, first, second) // idempotent: one base row per engine
}
