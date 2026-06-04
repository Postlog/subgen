//go:build integration

package users_test

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/repository/dbtest"
	"github.com/postlog/subgen/internal/repository/nodes"
	"github.com/postlog/subgen/internal/repository/users"
)

// TestRepository_Delete has effectively one behaviour worth pinning: deleting a user
// removes the row and CASCADEs its user_connections (the inbound it pointed at stays).
// A delete of a missing id is a no-op (no error). Straight-line, no table.
func TestRepository_Delete(t *testing.T) {
	t.Parallel()
	db := dbtest.OpenDB(t)
	nodeRepo := nodes.New(db)
	seed := dbtest.SeedNode(t, nodeRepo)
	repo := users.New(db)

	u := &entity.User{Name: "alice", SubID: "sub-alice",
		Connections: []entity.Connection{{InboundID: seed.Smart.ID}, {InboundID: seed.Force.ID}}}
	require.NoError(t, repo.Create(t.Context(), u))

	// Deleting a non-existent id is a no-op, not an error.
	require.NoError(t, repo.Delete(t.Context(), 99999))

	require.NoError(t, repo.Delete(t.Context(), u.ID))

	// The user is gone …
	_, err := repo.Get(t.Context(), u.ID)
	require.ErrorIs(t, err, sql.ErrNoRows)

	// … its connections cascaded …
	var conns int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM user_connections WHERE user_id=?`, u.ID).Scan(&conns))
	assert.Zero(t, conns)

	// … but the referenced inbound (and its node) survive.
	gotNode, err := nodeRepo.Get(t.Context(), seed.NodeID)
	require.NoError(t, err)
	assert.Len(t, gotNode.Inbounds, 2)
}
