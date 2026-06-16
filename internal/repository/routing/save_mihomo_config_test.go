//go:build integration

package routing_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/mihomo"
	"github.com/postlog/subgen/internal/repository/dbtest"
	"github.com/postlog/subgen/internal/repository/nodes"
	"github.com/postlog/subgen/internal/repository/routing"
)

// SaveMihomoConfig is the single atomic writer for the whole mihomo config (groups +
// members + rules + providers + base_yaml). These cases cover the load-bearing
// behaviour: group references carried as a 0-based INDEX resolve to the persisted
// group id; inbound references carry the real node_inbounds id; a duplicate
// rule-provider name maps to the sentinel AND rolls the whole transaction back; and a
// second save fully replaces the first.
func TestRepository_SaveMihomoConfig(t *testing.T) {
	t.Parallel()

	t.Run("success.index_and_inbound_resolution", func(t *testing.T) {
		t.Parallel()
		db := dbtest.OpenDB(t)
		seed := dbtest.SeedNode(t, nodes.New(db))
		repo := routing.New(db)
		cfg := dbtest.SeedConfig(t, db)

		// Two groups: group[0] "exit" holds an inbound member; group[1] "top" holds a
		// member referencing group[0] by INDEX 0. A rule MATCHes to group index 1.
		groups := []mihomo.GroupDraft{
			{Name: "exit", Type: mihomo.GroupSelect, Members: []mihomo.RefDraft{
				{Kind: mihomo.PolicyInbound, InboundID: dbtest.Ptr(seed.Smart.ID)},
			}},
			{Name: "top", Type: mihomo.GroupSelect, Members: []mihomo.RefDraft{
				{Kind: mihomo.PolicyGroup, GroupIdx: dbtest.Ptr(0)}, // index of "exit"
				{Kind: mihomo.PolicyDirect},
			}},
		}
		rules := []mihomo.RuleDraft{
			{
				Type: mihomo.RuleDomainSuffix, Value: dbtest.Ptr("example.com"),
				Target: mihomo.RefDraft{Kind: mihomo.PolicyInbound, InboundID: dbtest.Ptr(seed.Force.ID)},
			},
			{
				Type:   mihomo.RuleMatch,
				Target: mihomo.RefDraft{Kind: mihomo.PolicyGroup, GroupIdx: dbtest.Ptr(1)},
			}, // index of "top"
		}

		require.NoError(t, repo.SaveMihomoConfig(t.Context(), cfg, dbtest.Draft(rules, groups, nil, "mixed-port: 7890",
			mihomo.Profile{Title: "My VPN", Filename: "vpn.yaml", UpdateInterval: 3})))

		// Read groups back: ids are now real; the group member's GroupID was rewritten
		// from index 0 to the persisted id of "exit".
		gotGroups, err := repo.ProxyGroups(t.Context(), cfg)
		require.NoError(t, err)
		require.Len(t, gotGroups, 2)

		exit, top := gotGroups[0], gotGroups[1]
		require.Equal(t, "exit", exit.Name)
		require.Equal(t, "top", top.Name)

		// exit's inbound member resolves to the real inbound id.
		require.Len(t, exit.Members, 1)
		assert.Equal(t, mihomo.PolicyInbound, exit.Members[0].Kind)
		require.NotNil(t, exit.Members[0].InboundID)
		assert.Equal(t, seed.Smart.ID, *exit.Members[0].InboundID)

		// top's first member points at exit's *persisted* id (not the index 0).
		require.Len(t, top.Members, 2)
		assert.Equal(t, mihomo.PolicyGroup, top.Members[0].Kind)
		require.NotNil(t, top.Members[0].GroupID)
		assert.Equal(t, exit.ID, *top.Members[0].GroupID)
		assert.Equal(t, mihomo.PolicyDirect, top.Members[1].Kind)

		// Read rules back: the inbound target resolves; the group target points at top's id.
		gotRules, err := repo.Rules(t.Context(), cfg)
		require.NoError(t, err)
		require.Len(t, gotRules, 2)

		assert.Equal(t, mihomo.RuleDomainSuffix, gotRules[0].Type)
		require.NotNil(t, gotRules[0].Value)
		assert.Equal(t, "example.com", *gotRules[0].Value)
		require.NotNil(t, gotRules[0].Target.InboundID)
		assert.Equal(t, seed.Force.ID, *gotRules[0].Target.InboundID)

		assert.Equal(t, mihomo.RuleMatch, gotRules[1].Type)
		require.Equal(t, mihomo.PolicyGroup, gotRules[1].Target.Kind)
		require.NotNil(t, gotRules[1].Target.GroupID)
		assert.Equal(t, top.ID, *gotRules[1].Target.GroupID)

		// base_yaml went into mihomo_settings.
		base, err := repo.Setting(t.Context(), cfg, "base_yaml")
		require.NoError(t, err)
		assert.Equal(t, "mixed-port: 7890", base)

		// the profile row round-trips.
		prof, err := repo.Profile(t.Context(), cfg)
		require.NoError(t, err)
		assert.Equal(t, mihomo.Profile{Title: "My VPN", Filename: "vpn.yaml", UpdateInterval: 3}, prof)
	})

	t.Run("success.replaces_previous", func(t *testing.T) {
		t.Parallel()
		db := dbtest.OpenDB(t)
		seed := dbtest.SeedNode(t, nodes.New(db))
		repo := routing.New(db)
		cfg := dbtest.SeedConfig(t, db)

		// First save: one group, one rule, one provider, a base, a profile.
		require.NoError(t, repo.SaveMihomoConfig(t.Context(), cfg, dbtest.Draft(
			[]mihomo.RuleDraft{dbtest.RuleToInbound(seed.Smart.ID)},
			[]mihomo.GroupDraft{dbtest.GroupWithInbound("old", seed.Smart.ID)},
			[]mihomo.RuleProvider{{Name: "old-rp", Behavior: "domain", Format: "yaml", URL: "http://x"}},
			"base-old", mihomo.Profile{Title: "Old", Filename: "old.yaml", UpdateInterval: 2},
		)))

		// Second save: a different, smaller config — must fully replace the first.
		require.NoError(t, repo.SaveMihomoConfig(t.Context(), cfg, dbtest.Draft(
			nil,
			[]mihomo.GroupDraft{{Name: "new", Type: mihomo.GroupSelect, Members: []mihomo.RefDraft{{Kind: mihomo.PolicyDirect}}}},
			nil,
			"base-new", mihomo.Profile{Title: "New", Filename: "new.yaml", UpdateInterval: 9},
		)))

		gotRules, err := repo.Rules(t.Context(), cfg)
		require.NoError(t, err)
		assert.Empty(t, gotRules) // the old rule is gone

		gotGroups, err := repo.ProxyGroups(t.Context(), cfg)
		require.NoError(t, err)
		require.Len(t, gotGroups, 1)
		assert.Equal(t, "new", gotGroups[0].Name) // "old" replaced

		gotRPs, err := repo.RuleProviders(t.Context(), cfg)
		require.NoError(t, err)
		assert.Empty(t, gotRPs) // the old provider is gone

		base, err := repo.Setting(t.Context(), cfg, "base_yaml")
		require.NoError(t, err)
		assert.Equal(t, "base-new", base) // base upserted

		prof, err := repo.Profile(t.Context(), cfg)
		require.NoError(t, err)
		assert.Equal(t, mihomo.Profile{Title: "New", Filename: "new.yaml", UpdateInterval: 9}, prof) // profile upserted
	})

	t.Run("error.duplicate_rule_provider_rolls_back", func(t *testing.T) {
		t.Parallel()
		db := dbtest.OpenDB(t)
		seed := dbtest.SeedNode(t, nodes.New(db))
		repo := routing.New(db)
		cfg := dbtest.SeedConfig(t, db)

		// Seed a committed config so we can prove the failed save leaves it untouched.
		require.NoError(t, repo.SaveMihomoConfig(t.Context(), cfg, dbtest.Draft(
			[]mihomo.RuleDraft{dbtest.RuleToInbound(seed.Smart.ID)},
			[]mihomo.GroupDraft{dbtest.GroupWithInbound("keep", seed.Smart.ID)},
			[]mihomo.RuleProvider{{Name: "rp-keep", Behavior: "domain", Format: "yaml", URL: "http://keep"}},
			"base-keep", mihomo.Profile{},
		)))

		// Now attempt a save with two providers sharing the UNIQUE(config_id,name).
		err := repo.SaveMihomoConfig(t.Context(), cfg, dbtest.Draft(
			nil,
			[]mihomo.GroupDraft{{Name: "wont-stick", Type: mihomo.GroupSelect, Members: []mihomo.RefDraft{{Kind: mihomo.PolicyDirect}}}},
			[]mihomo.RuleProvider{
				{Name: "dup", Behavior: "domain", Format: "yaml", URL: "http://a"},
				{Name: "dup", Behavior: "ipcidr", Format: "yaml", URL: "http://b"},
			},
			"base-wont-stick", mihomo.Profile{},
		))
		require.ErrorIs(t, err, entity.ErrRuleProviderNameTaken)

		// The transaction rolled back: the original config is intact, none of the
		// attempted rows leaked.
		gotGroups, err := repo.ProxyGroups(t.Context(), cfg)
		require.NoError(t, err)
		require.Len(t, gotGroups, 1)
		assert.Equal(t, "keep", gotGroups[0].Name)

		gotRules, err := repo.Rules(t.Context(), cfg)
		require.NoError(t, err)
		assert.Len(t, gotRules, 1)

		gotRPs, err := repo.RuleProviders(t.Context(), cfg)
		require.NoError(t, err)
		require.Len(t, gotRPs, 1)
		assert.Equal(t, "rp-keep", gotRPs[0].Name)

		base, err := repo.Setting(t.Context(), cfg, "base_yaml")
		require.NoError(t, err)
		assert.Equal(t, "base-keep", base)
	})

	t.Run("success.logical_rule_condition_tree_roundtrip", func(t *testing.T) {
		t.Parallel()
		db := dbtest.OpenDB(t)
		repo := routing.New(db)
		cfg := dbtest.SeedConfig(t, db)

		// A logical rule AND( NETWORK=UDP, OR( DST-PORT=443, RULE-SET[idx 0] ) ) → DIRECT,
		// then MATCH. The RULE-SET sub-condition references the provider by array index; on
		// read it must resolve to the persisted provider id, and the tree must round-trip.
		provs := []mihomo.RuleProvider{{Name: "ads", Behavior: "domain", Format: "mrs", URL: "http://x"}}
		rules := []mihomo.RuleDraft{
			{Type: mihomo.RuleAnd, Target: mihomo.RefDraft{Kind: mihomo.PolicyDirect}, Conditions: []mihomo.ConditionDraft{
				{Type: mihomo.RuleNetwork, Value: dbtest.Ptr("UDP")},
				{Type: mihomo.RuleOr, Conditions: []mihomo.ConditionDraft{
					{Type: mihomo.RuleDstPort, Value: dbtest.Ptr("443")},
					{Type: mihomo.RuleRuleSet, ProviderIdx: dbtest.Ptr(0)},
				}},
			}},
			{Type: mihomo.RuleMatch, Target: mihomo.RefDraft{Kind: mihomo.PolicyDirect}},
		}

		require.NoError(t, repo.SaveMihomoConfig(t.Context(), cfg, dbtest.Draft(rules, nil, provs, "mode: rule",
			mihomo.Profile{Title: "T", Filename: "t.yaml", UpdateInterval: 1})))

		gotRPs, err := repo.RuleProviders(t.Context(), cfg)
		require.NoError(t, err)
		require.Len(t, gotRPs, 1)

		gotRules, err := repo.Rules(t.Context(), cfg)
		require.NoError(t, err)
		require.Len(t, gotRules, 2)

		and := gotRules[0]
		assert.Equal(t, mihomo.RuleAnd, and.Type)
		require.Len(t, and.Conditions, 2)

		assert.Equal(t, mihomo.RuleNetwork, and.Conditions[0].Type)
		require.NotNil(t, and.Conditions[0].Value)
		assert.Equal(t, "UDP", *and.Conditions[0].Value)

		or := and.Conditions[1]
		assert.Equal(t, mihomo.RuleOr, or.Type)
		require.Len(t, or.Conditions, 2)

		assert.Equal(t, mihomo.RuleDstPort, or.Conditions[0].Type)
		require.NotNil(t, or.Conditions[0].Value)
		assert.Equal(t, "443", *or.Conditions[0].Value)

		// The nested RULE-SET sub-condition resolves to the persisted provider id.
		rs := or.Conditions[1]
		assert.Equal(t, mihomo.RuleRuleSet, rs.Type)
		require.NotNil(t, rs.ProviderID)
		assert.Equal(t, gotRPs[0].ID, *rs.ProviderID)

		assert.Equal(t, mihomo.RuleMatch, gotRules[1].Type)
		assert.Empty(t, gotRules[1].Conditions)

		// A second save fully replaces the tree — the old conditions cascade away with the
		// old rules (rule_id FK), leaving no orphans.
		require.NoError(t, repo.SaveMihomoConfig(t.Context(), cfg, dbtest.Draft(
			[]mihomo.RuleDraft{{Type: mihomo.RuleMatch, Target: mihomo.RefDraft{Kind: mihomo.PolicyDirect}}},
			nil, nil, "mode: rule", mihomo.Profile{Title: "T", Filename: "t.yaml", UpdateInterval: 1},
		)))

		var conds int
		require.NoError(t, db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM mihomo_rule_conditions`).Scan(&conds))
		assert.Zero(t, conds)
	})

	t.Run("error.group_index_out_of_range", func(t *testing.T) {
		t.Parallel()
		db := dbtest.OpenDB(t)
		repo := routing.New(db)
		cfg := dbtest.SeedConfig(t, db)

		// A rule targeting group index 5 when no groups exist: refColumns rejects it and
		// the transaction rolls back (nothing persisted).
		err := repo.SaveMihomoConfig(t.Context(), cfg, dbtest.Draft(
			[]mihomo.RuleDraft{{
				Type:   mihomo.RuleMatch,
				Target: mihomo.RefDraft{Kind: mihomo.PolicyGroup, GroupIdx: dbtest.Ptr(5)},
			}},
			nil, nil, "", mihomo.Profile{},
		))
		require.ErrorContains(t, err, "group ref index out of range")

		gotRules, err := repo.Rules(t.Context(), cfg)
		require.NoError(t, err)
		assert.Empty(t, gotRules)
	})
}
