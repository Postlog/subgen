//go:build integration

package routing_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/mihomo"
	"github.com/postlog/subgen/internal/repository/dbtest"
	"github.com/postlog/subgen/internal/repository/routing"
)

// RuleProviders reads back all rule-providers ordered by name, with the mirror flag
// round-tripped from its integer column. It can be populated by SaveMihomoConfig or
// SetRuleProvider; here SaveMihomoConfig seeds the set.
func TestRepository_RuleProviders(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		got, err := routing.New(dbtest.OpenDB(t)).RuleProviders(t.Context(), 0)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("success.order_and_mirror_flag", func(t *testing.T) {
		t.Parallel()
		db := dbtest.OpenDB(t)
		repo := routing.New(db)
		cfg := dbtest.SeedConfig(t, db)

		// Inserted out of name order ("zeta" before "alpha") to prove ORDER BY name.
		want := []mihomo.RuleProvider{
			{Name: "zeta", Behavior: "ipcidr", Format: "mrs", Mirror: false, URL: "http://z", Interval: 86400},
			{Name: "alpha", Behavior: "domain", Format: "yaml", Mirror: true, URL: "http://a", Interval: 3600, MirrorInterval: 600},
		}
		require.NoError(t, repo.SaveMihomoConfig(t.Context(), cfg, dbtest.Draft(nil, nil, want, "", mihomo.Profile{})))

		got, err := repo.RuleProviders(t.Context(), cfg)
		require.NoError(t, err)
		require.Len(t, got, 2)

		// Ordered by name: alpha first.
		assert.Equal(t, "alpha", got[0].Name)
		assert.Equal(t, "zeta", got[1].Name)

		// alpha round-trips every field, mirror=true. ID is assigned by the DB.
		require.NotZero(t, got[0].ID)
		assert.Equal(t, mihomo.RuleProvider{
			ID:   got[0].ID,
			Name: "alpha", Behavior: "domain", Format: "yaml", Mirror: true,
			URL: "http://a", Interval: 3600, MirrorInterval: 600,
		}, got[0])
		// zeta has mirror=false.
		assert.False(t, got[1].Mirror)
	})
}
