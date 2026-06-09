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

// ProxyGroups reads back the operator-defined groups (ordered by position) with their
// ordered members (typed PolicyRefs). Populated only through SaveMihomoConfig.
func TestRepository_ProxyGroups(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		got, err := routing.New(dbtest.OpenDB(t)).ProxyGroups(t.Context(), 0)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("success.fields_order_and_members", func(t *testing.T) {
		t.Parallel()
		db := dbtest.OpenDB(t)
		seed := dbtest.SeedNode(t, nodes.New(db))
		repo := routing.New(db)
		cfg := dbtest.SeedConfig(t, db)

		// group[0] "auto" is a url-test with health-check fields and lazy set; its members
		// are (in order) an inbound and a built-in direct. group[1] "pick" references
		// group[0] by index 0.
		groups := []mihomo.ProxyGroup{
			{
				Name: "auto", Type: mihomo.GroupURLTest,
				URL: "http://gstatic/generate_204", Interval: 300, Tolerance: 50, Lazy: true,
				Members: []mihomo.PolicyRef{
					{Kind: mihomo.PolicyInbound, InboundID: dbtest.Ptr(seed.Smart.ID)},
					{Kind: mihomo.PolicyDirect},
				},
			},
			{
				Name: "pick", Type: mihomo.GroupSelect,
				Members: []mihomo.PolicyRef{{Kind: mihomo.PolicyGroup, GroupID: dbtest.Ptr(int64(0))}},
			},
		}
		require.NoError(t, repo.SaveMihomoConfig(t.Context(), cfg, nil, groups, nil, "", mihomo.Profile{}))

		got, err := repo.ProxyGroups(t.Context(), cfg)
		require.NoError(t, err)
		require.Len(t, got, 2)

		// Ordered by position; ids assigned.
		auto, pick := got[0], got[1]
		assert.Equal(t, 0, auto.Position)
		assert.Equal(t, 1, pick.Position)
		assert.NotZero(t, auto.ID)

		// All scalar fields round-trip, including the bool lazy.
		assert.Equal(t, "auto", auto.Name)
		assert.Equal(t, mihomo.GroupURLTest, auto.Type)
		assert.Equal(t, "http://gstatic/generate_204", auto.URL)
		assert.Equal(t, 300, auto.Interval)
		assert.Equal(t, 50, auto.Tolerance)
		assert.True(t, auto.Lazy)

		// Members come back in position order with their typed refs.
		require.Len(t, auto.Members, 2)
		assert.Equal(t, mihomo.PolicyInbound, auto.Members[0].Kind)
		require.NotNil(t, auto.Members[0].InboundID)
		assert.Equal(t, seed.Smart.ID, *auto.Members[0].InboundID)
		assert.Equal(t, mihomo.PolicyDirect, auto.Members[1].Kind)

		// pick's member references auto's persisted id (index 0 was resolved at save).
		require.Len(t, pick.Members, 1)
		assert.Equal(t, mihomo.PolicyGroup, pick.Members[0].Kind)
		require.NotNil(t, pick.Members[0].GroupID)
		assert.Equal(t, auto.ID, *pick.Members[0].GroupID)
	})
}
