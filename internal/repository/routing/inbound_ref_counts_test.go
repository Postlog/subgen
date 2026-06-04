//go:build integration

package routing_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/mihomo"
	"github.com/postlog/subgen/internal/repository/dbtest"
	"github.com/postlog/subgen/internal/repository/nodes"
	"github.com/postlog/subgen/internal/repository/routing"
)

// InboundRefCounts sums, per inbound id, the routing-rule + proxy-group-member
// references. These cases save a config that references "smart" from BOTH a rule and a
// group member (so its count is 2), leaves "force" unreferenced (absent from the map),
// and queries an unknown id (also absent).
func TestRepository_InboundRefCounts(t *testing.T) {
	tt := []struct {
		name string

		// arrange saves the mihomo config that establishes the references; query is the
		// inbound-id list; want is the expected non-zero counts.
		arrange func(t *testing.T, repo *routing.Repository, seed dbtest.SeededNode)
		query   func(seed dbtest.SeededNode) []int64
		want    func(seed dbtest.SeededNode) map[int64]int
	}{
		{
			name:  "empty.no_references",
			query: func(seed dbtest.SeededNode) []int64 { return []int64{seed.Smart.ID, seed.Force.ID} },
			want:  func(seed dbtest.SeededNode) map[int64]int { return map[int64]int{} },
		},
		{
			name: "success.rule_plus_member_combined",
			arrange: func(t *testing.T, repo *routing.Repository, seed dbtest.SeededNode) {
				// "smart" referenced by one rule AND one group member → count 2.
				require.NoError(t, repo.SaveMihomoConfig(t.Context(),
					[]mihomo.RoutingRule{dbtest.RuleToInbound(seed.Smart.ID)},
					[]mihomo.ProxyGroup{dbtest.GroupWithInbound("sel", seed.Smart.ID)},
					nil, ""))
			},
			query: func(seed dbtest.SeededNode) []int64 { return []int64{seed.Smart.ID, seed.Force.ID} },
			want:  func(seed dbtest.SeededNode) map[int64]int { return map[int64]int{seed.Smart.ID: 2} },
		},
		{
			name: "success.omits_zero_and_unknown",
			arrange: func(t *testing.T, repo *routing.Repository, seed dbtest.SeededNode) {
				require.NoError(t, repo.SaveMihomoConfig(t.Context(),
					[]mihomo.RoutingRule{dbtest.RuleToInbound(seed.Force.ID)}, nil, nil, ""))
			},
			// smart has 0, id 99999 doesn't exist — both absent; only force counts.
			query: func(seed dbtest.SeededNode) []int64 { return []int64{seed.Smart.ID, seed.Force.ID, 99999} },
			want:  func(seed dbtest.SeededNode) map[int64]int { return map[int64]int{seed.Force.ID: 1} },
		},
	}

	t.Parallel()
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := dbtest.OpenDB(t)
			seed := dbtest.SeedNode(t, nodes.New(db))
			repo := routing.New(db)

			if tc.arrange != nil {
				tc.arrange(t, repo, seed)
			}

			got, err := repo.InboundRefCounts(t.Context(), tc.query(seed))
			require.NoError(t, err)
			assert.Equal(t, tc.want(seed), got)
		})
	}
}
