//go:build integration

package users_test

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/repository/dbtest"
	"github.com/postlog/subgen/internal/repository/users"
)

// TestRepository_SubIDs has one behaviour: it returns every user's sub_id (empty on a
// fresh store). Straight-line, no table. No node fixture is needed — users without
// connections are valid.
func TestRepository_SubIDs(t *testing.T) {
	t.Parallel()
	repo := users.New(dbtest.OpenDB(t))

	// Empty store: no sub ids.
	empty, err := repo.SubIDs(t.Context())
	require.NoError(t, err)
	require.Empty(t, empty)

	require.NoError(t, repo.Create(t.Context(), &entity.User{Name: "alice", SubID: "sub-alice"}))
	require.NoError(t, repo.Create(t.Context(), &entity.User{Name: "bob", SubID: "sub-bob"}))

	got, err := repo.SubIDs(t.Context())
	require.NoError(t, err)

	sort.Strings(got)
	assert.Equal(t, []string{"sub-alice", "sub-bob"}, got)
}
