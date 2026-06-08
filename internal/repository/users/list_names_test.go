//go:build integration

package users_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/repository/dbtest"
	"github.com/postlog/subgen/internal/repository/nodes"
	"github.com/postlog/subgen/internal/repository/users"
)

// TestRepository_ListNames has one meaningful scenario (empty store, then users by
// name as id+name only — no connections hydrated), so it is straight-line.
func TestRepository_ListNames(t *testing.T) {
	t.Parallel()
	db := dbtest.OpenDB(t)
	seed := dbtest.SeedNode(t, nodes.New(db))
	repo := users.New(db)

	empty, err := repo.ListNames(t.Context())
	require.NoError(t, err)
	require.Empty(t, empty)

	// "zoe" has a connection; ListNames must still return no connections (cheap lookup).
	require.NoError(t, repo.Create(t.Context(), &entity.User{Name: "zoe", SubID: "s-zoe",
		Connections: []entity.Connection{{InboundID: seed.Smart.ID}}}))
	require.NoError(t, repo.Create(t.Context(), &entity.User{Name: "amy", SubID: "s-amy"}))

	list, err := repo.ListNames(t.Context())
	require.NoError(t, err)
	require.Len(t, list, 2)

	// Ordered by name, id+name populated, connections left empty.
	assert.Equal(t, "amy", list[0].Name)
	assert.NotZero(t, list[0].ID)
	assert.Equal(t, "zoe", list[1].Name)
	assert.Empty(t, list[1].Connections)
}
