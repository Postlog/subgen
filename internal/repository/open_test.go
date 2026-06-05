//go:build integration

// Package repository_test holds the integration test for the Open bootstrap. It runs
// against a REAL temporary SQLite database (no external services): each subtest opens
// its own fresh file under t.TempDir() via repository.Open, which applies the embedded
// schema and turns foreign_keys ON. Open is exported, so this is a black-box test;
// the shared open/seed helpers live in the dbtest support package.
//
// Run: go test -tags integration ./internal/repository/...
package repository_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/repository"
	"github.com/postlog/subgen/internal/repository/dbtest"
	"github.com/postlog/subgen/internal/repository/nodes"
	"github.com/postlog/subgen/internal/repository/routing"
	"github.com/postlog/subgen/internal/repository/users"
)

// TestOpen exercises the bootstrap: a freshly opened db has the schema applied (so
// every repository read succeeds and returns nothing — no defaults are seeded),
// foreign_keys is ON (a constraint-violating write is rejected by the engine), and a
// re-open of the same path is idempotent and keeps the data.
func TestOpen(t *testing.T) {
	t.Parallel()

	t.Run("fresh.repos_empty", func(t *testing.T) {
		t.Parallel()
		db := dbtest.OpenDB(t)

		// Every read works against the applied schema and a fresh store yields nothing.
		nodesList, err := nodes.New(db).List(t.Context())
		require.NoError(t, err)
		assert.Empty(t, nodesList)

		usersList, err := users.New(db).List(t.Context())
		require.NoError(t, err)
		assert.Empty(t, usersList)

		subIDs, err := users.New(db).SubIDs(t.Context())
		require.NoError(t, err)
		assert.Empty(t, subIDs)

		rt := routing.New(db)

		rules, err := rt.Rules(t.Context(), 0)
		require.NoError(t, err)
		assert.Empty(t, rules)

		groups, err := rt.ProxyGroups(t.Context(), 0)
		require.NoError(t, err)
		assert.Empty(t, groups)

		providers, err := rt.RuleProviders(t.Context(), 0)
		require.NoError(t, err)
		assert.Empty(t, providers)

		base, err := rt.Setting(t.Context(), 0, "base_yaml")
		require.NoError(t, err)
		assert.Empty(t, base)
	})

	t.Run("foreign_keys.on", func(t *testing.T) {
		t.Parallel()
		db := dbtest.OpenDB(t)

		// A user_connections row referencing a non-existent inbound violates the FK to
		// node_inbounds. With foreign_keys ON the engine rejects it; with the pragma off
		// this would silently insert.
		_, err := db.Exec(`INSERT INTO user_connections(user_id,inbound_id,created_at) VALUES(1,999,0)`)
		require.Error(t, err)
		assert.ErrorContains(t, err, "FOREIGN KEY")
	})

	t.Run("idempotent.reopen_same_file", func(t *testing.T) {
		t.Parallel()
		// Opening the same path twice re-applies the schema (CREATE … IF NOT EXISTS); the
		// second Open must not error and must keep the data written via the first.
		path := t.TempDir() + "/subgen.db"

		db1, err := repository.Open(t.Context(), path)
		require.NoError(t, err)

		_, err = db1.Exec(`INSERT INTO subscription_configs(id,user_id,kind,created_at) VALUES(1,NULL,'mihomo',0)`)
		require.NoError(t, err)
		_, err = db1.Exec(`INSERT INTO mihomo_settings(config_id,key,value) VALUES(1,'base_yaml','hello')`)
		require.NoError(t, err)
		require.NoError(t, db1.Close())

		db2, err := repository.Open(t.Context(), path)
		require.NoError(t, err)
		t.Cleanup(func() { _ = db2.Close() })

		var got string
		require.NoError(t, db2.QueryRow(`SELECT value FROM mihomo_settings WHERE config_id=1 AND key='base_yaml'`).Scan(&got))
		assert.Equal(t, "hello", got)
	})
}
