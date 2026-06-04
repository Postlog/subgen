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

func TestRepository_Create(t *testing.T) {
	tt := []struct {
		name string

		// build returns the user to create, given the seeded node's inbound ids (so a
		// connection can reference a real node_inbounds row). seedDup, when set, is a
		// user created first to force a UNIQUE collision (on name and/or sub_id).
		build   func(seed dbtest.SeededNode) *entity.User
		seedDup func(seed dbtest.SeededNode) *entity.User

		err     error
		wantErr bool // for the FK violation case (no domain sentinel)
	}{
		{
			name: "success.with_connections",
			build: func(seed dbtest.SeededNode) *entity.User {
				return &entity.User{Name: "alice", SubID: "sub-alice",
					Connections: []entity.Connection{{InboundID: seed.Smart.ID}, {InboundID: seed.Force.ID}}}
			},
		},
		{
			name: "success.no_connections",
			build: func(seed dbtest.SeededNode) *entity.User {
				return &entity.User{Name: "bob", SubID: "sub-bob"}
			},
		},
		{
			name: "error.duplicate_name",
			seedDup: func(seed dbtest.SeededNode) *entity.User {
				return &entity.User{Name: "alice", SubID: "sub-1"}
			},
			build: func(seed dbtest.SeededNode) *entity.User {
				return &entity.User{Name: "alice", SubID: "sub-2"} // same name, fresh sub_id
			},
			err: entity.ErrNameTaken,
		},
		{
			name: "error.duplicate_sub_id",
			seedDup: func(seed dbtest.SeededNode) *entity.User {
				return &entity.User{Name: "carol", SubID: "sub-shared"}
			},
			build: func(seed dbtest.SeededNode) *entity.User {
				return &entity.User{Name: "dave", SubID: "sub-shared"} // fresh name, same sub_id
			},
			err: entity.ErrNameTaken, // both UNIQUE(name) and UNIQUE(sub_id) map to ErrNameTaken
		},
		{
			name: "error.connection_to_unknown_inbound",
			build: func(seed dbtest.SeededNode) *entity.User {
				return &entity.User{Name: "eve", SubID: "sub-eve",
					Connections: []entity.Connection{{InboundID: 999999}}} // no such node_inbounds row
			},
			wantErr: true, // FK to node_inbounds violated; the whole tx rolls back
		},
	}

	t.Parallel()
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := dbtest.OpenDB(t)
			seed := dbtest.SeedNode(t, nodes.New(db))
			repo := users.New(db)

			if tc.seedDup != nil {
				require.NoError(t, repo.Create(t.Context(), tc.seedDup(seed)))
			}

			u := tc.build(seed)
			err := repo.Create(t.Context(), u)

			if tc.wantErr {
				require.Error(t, err)
				// The FK violation rolls the transaction back: nothing is persisted (we
				// assert on the DB, not on u.ID — Create assigns u.ID from the users
				// insert before the connection insert fails, and a rollback does not reset
				// that in-memory field).
				list, listErr := repo.List(t.Context())
				require.NoError(t, listErr)
				assert.Empty(t, list) // "eve" never landed; no other user was created
				return
			}

			require.ErrorIs(t, err, tc.err)
			if tc.err != nil {
				return
			}

			// Success: ids assigned to the user and to each connection; it round-trips.
			require.NotZero(t, u.ID)
			for _, c := range u.Connections {
				assert.NotZero(t, c.ID)
				assert.Equal(t, u.ID, c.UserID)
				assert.NotZero(t, c.CreatedAt)
			}

			got, err := repo.Get(t.Context(), u.ID)
			require.NoError(t, err)
			assert.Equal(t, u.Name, got.Name)
			assert.Equal(t, u.SubID, got.SubID)
			assert.Len(t, got.Connections, len(u.Connections))
		})
	}
}
