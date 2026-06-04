//go:build integration

package users_test

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/repository/dbtest"
	"github.com/postlog/subgen/internal/repository/nodes"
	"github.com/postlog/subgen/internal/repository/users"
)

func TestRepository_ReplaceConnections(t *testing.T) {
	tt := []struct {
		name string

		// want returns the inbound-id set to replace with, given the seeded node. The
		// user starts bound to just "smart".
		want func(seed dbtest.SeededNode) []int64
		// expect returns the inbound ids the user should end up bound to.
		expect func(seed dbtest.SeededNode) []int64
	}{
		{
			name:   "add_one",
			want:   func(seed dbtest.SeededNode) []int64 { return []int64{seed.Smart.ID, seed.Force.ID} },
			expect: func(seed dbtest.SeededNode) []int64 { return []int64{seed.Smart.ID, seed.Force.ID} },
		},
		{
			name:   "remove_all",
			want:   func(seed dbtest.SeededNode) []int64 { return nil },
			expect: func(seed dbtest.SeededNode) []int64 { return nil },
		},
		{
			name:   "swap",
			want:   func(seed dbtest.SeededNode) []int64 { return []int64{seed.Force.ID} },
			expect: func(seed dbtest.SeededNode) []int64 { return []int64{seed.Force.ID} },
		},
		{
			name: "idempotent.same_set",
			// Replacing with the existing set is a no-op (the want-but-have branch skips
			// the insert, so UNIQUE(user_id,inbound_id) is never hit).
			want:   func(seed dbtest.SeededNode) []int64 { return []int64{seed.Smart.ID} },
			expect: func(seed dbtest.SeededNode) []int64 { return []int64{seed.Smart.ID} },
		},
		{
			name:   "duplicates_in_input_collapse",
			want:   func(seed dbtest.SeededNode) []int64 { return []int64{seed.Force.ID, seed.Force.ID} },
			expect: func(seed dbtest.SeededNode) []int64 { return []int64{seed.Force.ID} },
		},
	}

	t.Parallel()
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := dbtest.OpenDB(t)
			seed := dbtest.SeedNode(t, nodes.New(db))
			repo := users.New(db)

			// Start every case from a user bound to exactly "smart".
			u := &entity.User{Name: "alice", SubID: "sub-alice",
				Connections: []entity.Connection{{InboundID: seed.Smart.ID}}}
			require.NoError(t, repo.Create(t.Context(), u))

			require.NoError(t, repo.ReplaceConnections(t.Context(), u.ID, tc.want(seed)))

			got, err := repo.Get(t.Context(), u.ID)
			require.NoError(t, err)

			var gotIDs []int64
			for _, c := range got.Connections {
				gotIDs = append(gotIDs, c.InboundID)
			}

			expect := tc.expect(seed)
			sort.Slice(gotIDs, func(i, j int) bool { return gotIDs[i] < gotIDs[j] })
			sort.Slice(expect, func(i, j int) bool { return expect[i] < expect[j] })
			assert.Equal(t, expect, gotIDs)
		})
	}
}
