//go:build integration

package configs_test

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/mihomo"
	"github.com/postlog/subgen/internal/repository/configs"
	"github.com/postlog/subgen/internal/repository/dbtest"
	"github.com/postlog/subgen/internal/repository/nodes"
	"github.com/postlog/subgen/internal/repository/routing"
	"github.com/postlog/subgen/internal/repository/users"
)

// CreateUserConfig snapshots the engine's base config into a new per-user config: the
// content is cloned (groups+members with remapped ids, rules, providers, settings) and
// independent of the base afterwards. A second create for the same user is rejected.
func TestRepository_CreateUserConfig(t *testing.T) {
	t.Parallel()

	t.Run("success.clones_base_content", func(t *testing.T) {
		t.Parallel()
		db := dbtest.OpenDB(t)
		seed := dbtest.SeedNode(t, nodes.New(db))
		rt := routing.New(db)
		repo := configs.New(db, rt)

		userID := seedUser(t, db, "alice", "sub-alice")

		// Base config: a group with an inbound member + a group ref, and a rule whose
		// target is that group; plus a provider and base YAML.
		baseID, err := repo.EnsureBaseConfigID(t.Context(), entity.ConfigKindMihomo)
		require.NoError(t, err)

		groups := []mihomo.GroupDraft{
			{Name: "exit", Type: mihomo.GroupSelect, Members: []mihomo.RefDraft{
				{Kind: mihomo.PolicyInbound, InboundID: dbtest.Ptr(seed.Smart.ID)},
			}},
			{Name: "top", Type: mihomo.GroupSelect, Members: []mihomo.RefDraft{
				{Kind: mihomo.PolicyGroup, GroupIdx: dbtest.Ptr(0)}, // → "exit"
			}},
		}
		rules := []mihomo.RuleDraft{
			{Type: mihomo.RuleMatch, Target: mihomo.RefDraft{Kind: mihomo.PolicyGroup, GroupIdx: dbtest.Ptr(1)}},
		}
		provs := []mihomo.RuleProvider{{Name: "ads", Behavior: "domain", Format: "yaml", URL: "http://ads"}}
		require.NoError(t, rt.SaveMihomoConfig(t.Context(), baseID, dbtest.Draft(rules, groups, provs, "dns: {}",
			mihomo.Profile{Title: "Base", Filename: "base.yaml", UpdateInterval: 4})))

		// Create the custom config.
		newID, err := repo.CreateUserConfig(t.Context(), userID, entity.ConfigKindMihomo)
		require.NoError(t, err)
		assert.NotEqual(t, baseID, newID)

		// Cloned content matches the base shape, with group refs remapped to the clone's ids.
		gotGroups, err := rt.ProxyGroups(t.Context(), newID)
		require.NoError(t, err)
		require.Len(t, gotGroups, 2)
		assert.Equal(t, "exit", gotGroups[0].Name)
		assert.Equal(t, "top", gotGroups[1].Name)

		require.Len(t, gotGroups[0].Members, 1)
		require.NotNil(t, gotGroups[0].Members[0].InboundID)
		assert.Equal(t, seed.Smart.ID, *gotGroups[0].Members[0].InboundID)

		require.Len(t, gotGroups[1].Members, 1)
		require.NotNil(t, gotGroups[1].Members[0].GroupID)
		assert.Equal(t, gotGroups[0].ID, *gotGroups[1].Members[0].GroupID) // remapped to the clone

		gotRules, err := rt.Rules(t.Context(), newID)
		require.NoError(t, err)
		require.Len(t, gotRules, 1)
		require.NotNil(t, gotRules[0].Target.GroupID)
		assert.Equal(t, gotGroups[1].ID, *gotRules[0].Target.GroupID) // remapped to the clone's "top"

		gotProvs, err := rt.RuleProviders(t.Context(), newID)
		require.NoError(t, err)
		require.Len(t, gotProvs, 1)
		assert.Equal(t, "ads", gotProvs[0].Name)

		base, err := rt.Setting(t.Context(), newID, "base_yaml")
		require.NoError(t, err)
		assert.Equal(t, "dns: {}", base)

		// the profile row is cloned too.
		prof, err := rt.Profile(t.Context(), newID)
		require.NoError(t, err)
		assert.Equal(t, mihomo.Profile{Title: "Base", Filename: "base.yaml", UpdateInterval: 4}, prof)
	})

	t.Run("success.independent_snapshot", func(t *testing.T) {
		t.Parallel()
		db := dbtest.OpenDB(t)
		rt := routing.New(db)
		repo := configs.New(db, rt)
		userID := seedUser(t, db, "bob", "sub-bob")

		baseID, err := repo.EnsureBaseConfigID(t.Context(), entity.ConfigKindMihomo)
		require.NoError(t, err)
		require.NoError(t, rt.SaveMihomoConfig(t.Context(), baseID, dbtest.Draft(nil, nil, nil, "base: v1",
			mihomo.Profile{Title: "v1", Filename: "v1.yaml", UpdateInterval: 1})))

		newID, err := repo.CreateUserConfig(t.Context(), userID, entity.ConfigKindMihomo)
		require.NoError(t, err)

		// Edit the base after cloning — the custom config must not change.
		require.NoError(t, rt.SaveMihomoConfig(t.Context(), baseID, dbtest.Draft(nil, nil, nil, "base: v2",
			mihomo.Profile{Title: "v2", Filename: "v2.yaml", UpdateInterval: 2})))

		got, err := rt.Setting(t.Context(), newID, "base_yaml")
		require.NoError(t, err)
		assert.Equal(t, "base: v1", got)

		// the cloned profile is likewise an independent snapshot.
		prof, err := rt.Profile(t.Context(), newID)
		require.NoError(t, err)
		assert.Equal(t, mihomo.Profile{Title: "v1", Filename: "v1.yaml", UpdateInterval: 1}, prof)
	})

	t.Run("success.empty_base", func(t *testing.T) {
		t.Parallel()
		db := dbtest.OpenDB(t)
		repo := configs.New(db, routing.New(db))
		userID := seedUser(t, db, "carol", "sub-carol")

		// No base saved yet — the custom config is created empty (no clone source).
		newID, err := repo.CreateUserConfig(t.Context(), userID, entity.ConfigKindMihomo)
		require.NoError(t, err)
		assert.NotZero(t, newID)
	})

	t.Run("error.already_exists", func(t *testing.T) {
		t.Parallel()
		db := dbtest.OpenDB(t)
		repo := configs.New(db, routing.New(db))
		userID := seedUser(t, db, "dave", "sub-dave")

		_, err := repo.CreateUserConfig(t.Context(), userID, entity.ConfigKindMihomo)
		require.NoError(t, err)

		_, err = repo.CreateUserConfig(t.Context(), userID, entity.ConfigKindMihomo)
		require.ErrorIs(t, err, entity.ErrUserConfigExists)
	})
}

// seedUser creates a user through the real repository and returns its id.
func seedUser(t *testing.T, db *sql.DB, name, subID string) int64 {
	t.Helper()

	u := &entity.User{Name: name, SubID: subID}
	require.NoError(t, users.New(db).Create(t.Context(), u))

	return u.ID
}
