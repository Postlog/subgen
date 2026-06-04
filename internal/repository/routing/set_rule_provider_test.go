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

// TestRepository_SetRuleProvider covers the upsert: a first call inserts the row, a
// second call with the same name (the PRIMARY KEY) updates every other column in
// place — there is one provider, with the latest values. Straight-line, no table.
func TestRepository_SetRuleProvider(t *testing.T) {
	t.Parallel()
	repo := routing.New(dbtest.OpenDB(t))

	require.NoError(t, repo.SetRuleProvider(t.Context(), mihomo.RuleProvider{
		Name: "geosite", Behavior: "domain", Format: "yaml", Mirror: false,
		URL: "http://old", Interval: 3600,
	}))

	// Same name → ON CONFLICT updates in place (mirror flips on, url/interval change).
	require.NoError(t, repo.SetRuleProvider(t.Context(), mihomo.RuleProvider{
		Name: "geosite", Behavior: "domain", Format: "mrs", Mirror: true,
		URL: "http://new", Interval: 7200, MirrorInterval: 900,
	}))

	got, err := repo.RuleProviders(t.Context())
	require.NoError(t, err)
	require.Len(t, got, 1) // upsert, not a second row

	assert.Equal(t, mihomo.RuleProvider{
		Name: "geosite", Behavior: "domain", Format: "mrs", Mirror: true,
		URL: "http://new", Interval: 7200, MirrorInterval: 900,
	}, got[0])
}
