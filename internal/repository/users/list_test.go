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

// TestRepository_List has effectively one meaningful scenario (an empty store, then
// two users ordered by name with their connections hydrated by the join), so it is
// straight-line rather than a table.
func TestRepository_List(t *testing.T) {
	t.Parallel()
	db := dbtest.OpenDB(t)
	seed := dbtest.SeedNode(t, nodes.New(db))
	repo := users.New(db)

	// Empty store first: List returns nothing.
	empty, err := repo.List(t.Context())
	require.NoError(t, err)
	require.Empty(t, empty)

	// "zoe" with two connections, "amy" with one — inserted out of order to prove the
	// ORDER BY name.
	require.NoError(t, repo.Create(t.Context(), &entity.User{Name: "zoe", SubID: "sub-zoe",
		Connections: []entity.Connection{{InboundID: seed.Smart.ID}, {InboundID: seed.Force.ID}}}))
	require.NoError(t, repo.Create(t.Context(), &entity.User{Name: "amy", SubID: "sub-amy",
		Connections: []entity.Connection{{InboundID: seed.Smart.ID}}}))

	list, err := repo.List(t.Context())
	require.NoError(t, err)
	require.Len(t, list, 2)

	// Ordered by name: amy before zoe, each carrying its own connections.
	assert.Equal(t, "amy", list[0].Name)
	assert.Len(t, list[0].Connections, 1)
	assert.Equal(t, "zoe", list[1].Name)
	assert.Len(t, list[1].Connections, 2)
	// Connections are hydrated with the joined node name.
	assert.Equal(t, "RU1", list[0].Connections[0].Node)
}
