//go:build integration

package nodes_test

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

func TestRepository_ConnectionCountsByInbound(t *testing.T) {
	tt := []struct {
		name string

		// arrange creates users/connections against the seeded node (via the sibling
		// users repository over the same handle); query is the list of inbound ids to
		// count; want is the expected non-zero counts.
		arrange func(t *testing.T, db *sql.DB, seed dbtest.SeededNode)
		query   func(seed dbtest.SeededNode) []int64
		want    func(seed dbtest.SeededNode) map[int64]int
	}{
		{
			name:  "empty.no_connections",
			query: func(seed dbtest.SeededNode) []int64 { return []int64{seed.Smart.ID, seed.Force.ID} },
			want:  func(seed dbtest.SeededNode) map[int64]int { return map[int64]int{} },
		},
		{
			name: "success.counts_per_inbound",
			arrange: func(t *testing.T, db *sql.DB, seed dbtest.SeededNode) {
				ur := users.New(db)
				// Two users on smart, one on force.
				for _, n := range []string{"a", "b"} {
					require.NoError(t, ur.Create(t.Context(), &entity.User{Name: n, SubID: "sub-" + n,
						Connections: []entity.Connection{{InboundID: seed.Smart.ID}}}))
				}
				require.NoError(t, ur.Create(t.Context(), &entity.User{Name: "c", SubID: "sub-c",
					Connections: []entity.Connection{{InboundID: seed.Force.ID}}}))
			},
			query: func(seed dbtest.SeededNode) []int64 { return []int64{seed.Smart.ID, seed.Force.ID} },
			want: func(seed dbtest.SeededNode) map[int64]int {
				return map[int64]int{seed.Smart.ID: 2, seed.Force.ID: 1}
			},
		},
		{
			name: "success.omits_zero_and_unknown",
			arrange: func(t *testing.T, db *sql.DB, seed dbtest.SeededNode) {
				require.NoError(t, users.New(db).Create(t.Context(), &entity.User{Name: "a", SubID: "sub-a",
					Connections: []entity.Connection{{InboundID: seed.Smart.ID}}}))
			},
			// force has 0, id 99999 doesn't exist — both must be absent from the map.
			query: func(seed dbtest.SeededNode) []int64 { return []int64{seed.Smart.ID, seed.Force.ID, 99999} },
			want:  func(seed dbtest.SeededNode) map[int64]int { return map[int64]int{seed.Smart.ID: 1} },
		},
	}

	t.Parallel()
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := dbtest.OpenDB(t)
			repo := nodes.New(db)
			seed := dbtest.SeedNode(t, repo)

			if tc.arrange != nil {
				tc.arrange(t, db, seed)
			}

			got, err := repo.ConnectionCountsByInbound(t.Context(), tc.query(seed))
			require.NoError(t, err)
			assert.Equal(t, tc.want(seed), got)
		})
	}
}
