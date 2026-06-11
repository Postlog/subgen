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

func TestRepository_Get(t *testing.T) {
	tt := []struct {
		name string

		seed    bool // create a user with connections and Get it; else Get a missing id
		wantErr error
	}{
		{name: "success.with_connections", seed: true},
		{name: "error.not_found", seed: false, wantErr: sql.ErrNoRows},
	}

	t.Parallel()
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := dbtest.OpenDB(t)
			seed := dbtest.SeedNode(t, nodes.New(db))
			repo := users.New(db)

			var id int64 = 4242 // a non-existent id for the not-found case

			var smartID int64
			if tc.seed {
				smartID = seed.Smart.ID
				u := &entity.User{Name: "alice", SubID: "sub-alice", Description: dbtest.Ptr("work laptop"),
					Connections: []entity.Connection{{InboundID: smartID}}}
				require.NoError(t, repo.Create(t.Context(), u))
				id = u.ID
			}

			got, err := repo.Get(t.Context(), id)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, "alice", got.Name)
			assert.Equal(t, "sub-alice", got.SubID)
			require.NotNil(t, got.Description)
			assert.Equal(t, "work laptop", *got.Description)
			assert.NotZero(t, got.CreatedAt)

			// The connection is hydrated with the joined node/inbound fields.
			require.Len(t, got.Connections, 1)
			c := got.Connections[0]
			assert.Equal(t, smartID, c.InboundID)
			assert.Equal(t, seed.NodeID, c.NodeID)
			assert.Equal(t, "RU1", c.Node)
			assert.Equal(t, "smart", c.Name)
			assert.Equal(t, 4433, c.Port)
		})
	}
}
