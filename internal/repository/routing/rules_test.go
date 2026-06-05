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

// Rules reads back the ordered routing rules with their typed PolicyRef target. It is
// populated only through SaveMihomoConfig, so these cases save a config and assert the
// read-back shape: order by position, no_resolve round-trip, and the target's
// reconstruction for each PolicyKind (built-in / inbound / group).
func TestRepository_Rules(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		got, err := routing.New(dbtest.OpenDB(t)).Rules(t.Context(), 0)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("success.order_kinds_and_no_resolve", func(t *testing.T) {
		t.Parallel()
		db := dbtest.OpenDB(t)
		seed := dbtest.SeedNode(t, nodes.New(db))
		repo := routing.New(db)
		cfg := dbtest.SeedConfig(t, db)

		groups := []mihomo.ProxyGroup{
			{Name: "g", Type: mihomo.GroupSelect, Members: []mihomo.PolicyRef{{Kind: mihomo.PolicyDirect}}},
		}
		// Saved in this slice order; positions 0..3 must come back in the same order.
		rules := []mihomo.RoutingRule{
			{Type: mihomo.RuleIPCIDR, Value: "10.0.0.0/8", NoResolve: true,
				Target: mihomo.PolicyRef{Kind: mihomo.PolicyDirect}},
			{Type: mihomo.RuleDomainSuffix, Value: "ex.com",
				Target: mihomo.PolicyRef{Kind: mihomo.PolicyInbound, InboundID: dbtest.Ptr(seed.Smart.ID)}},
			{Type: mihomo.RuleGeoIP, Value: "CN", NoResolve: false,
				Target: mihomo.PolicyRef{Kind: mihomo.PolicyGroup, GroupID: dbtest.Ptr(int64(0))}},
			{Type: mihomo.RuleMatch,
				Target: mihomo.PolicyRef{Kind: mihomo.PolicyReject}},
		}
		require.NoError(t, repo.SaveMihomoConfig(t.Context(), cfg, rules, groups, nil, ""))

		got, err := repo.Rules(t.Context(), cfg)
		require.NoError(t, err)
		require.Len(t, got, 4)

		// Positions are 0..3 in slice order.
		for i, r := range got {
			assert.Equal(t, i, r.Position)
			assert.NotZero(t, r.ID)
		}

		// [0] built-in direct, no_resolve true.
		assert.Equal(t, mihomo.RuleIPCIDR, got[0].Type)
		assert.True(t, got[0].NoResolve)
		assert.Equal(t, mihomo.PolicyDirect, got[0].Target.Kind)
		assert.Nil(t, got[0].Target.InboundID)
		assert.Nil(t, got[0].Target.GroupID)

		// [1] inbound target: InboundID set, GroupID nil, no_resolve default false.
		assert.False(t, got[1].NoResolve)
		assert.Equal(t, mihomo.PolicyInbound, got[1].Target.Kind)
		require.NotNil(t, got[1].Target.InboundID)
		assert.Equal(t, seed.Smart.ID, *got[1].Target.InboundID)
		assert.Nil(t, got[1].Target.GroupID)

		// [2] group target: GroupID set to the persisted group id, InboundID nil.
		groupsBack, err := repo.ProxyGroups(t.Context(), cfg)
		require.NoError(t, err)
		require.Len(t, groupsBack, 1)
		assert.Equal(t, mihomo.PolicyGroup, got[2].Target.Kind)
		require.NotNil(t, got[2].Target.GroupID)
		assert.Equal(t, groupsBack[0].ID, *got[2].Target.GroupID)
		assert.Nil(t, got[2].Target.InboundID)

		// [3] built-in reject MATCH.
		assert.Equal(t, mihomo.RuleMatch, got[3].Type)
		assert.Equal(t, mihomo.PolicyReject, got[3].Target.Kind)
	})
}
