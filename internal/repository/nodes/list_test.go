//go:build integration

package nodes_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/repository/dbtest"
	"github.com/postlog/subgen/internal/repository/nodes"
)

// TestRepository_List has effectively one meaningful scenario (an empty store, then
// two nodes each with their inbounds, ordered by name), so it is straight-line rather
// than a table.
func TestRepository_List(t *testing.T) {
	t.Parallel()
	repo := nodes.New(dbtest.OpenDB(t))

	// Empty store first: List returns nothing.
	empty, err := repo.List(t.Context())
	require.NoError(t, err)
	require.Empty(t, empty)

	_, err = repo.Create(t.Context(), entity.Node{Name: "ZZ", VPNHost: "zz", Token: "t",
		Inbounds: []entity.Inbound{{Name: "only", Port: 1}}})
	require.NoError(t, err)
	_, err = repo.Create(t.Context(), entity.Node{Name: "AA", VPNHost: "aa", Token: "t",
		Inbounds: []entity.Inbound{{Name: "x", Port: 2}, {Name: "y", Port: 3}}})
	require.NoError(t, err)

	list, err := repo.List(t.Context())
	require.NoError(t, err)
	require.Len(t, list, 2)

	// Ordered by name: AA before ZZ, each carrying its own inbounds.
	assert.Equal(t, "AA", list[0].Name)
	assert.Equal(t, "ZZ", list[1].Name)
	assert.Len(t, list[0].Inbounds, 2)
	assert.Len(t, list[1].Inbounds, 1)
}
