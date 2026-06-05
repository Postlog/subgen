//go:build integration

// Package dbtest is the shared support code for the repository integration tests.
// It opens a REAL temporary SQLite database (the schema applied, foreign_keys ON)
// and seeds common fixtures, so each per-method test file under
// internal/repository/{nodes,users,routing} can spin up an isolated store and the
// referenced rows it needs.
//
// It is built only under -tags integration. No import cycle: repository.Open imports
// only the migrations; the per-entity repositories (nodes/users/routing) import dberr,
// not repository — so dbtest may import all four, and the external *_test packages
// (nodes_test, users_test, …) may import dbtest.
package dbtest

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/mihomo"
	"github.com/postlog/subgen/internal/repository"
	"github.com/postlog/subgen/internal/repository/nodes"
)

// OpenDB opens a brand-new SQLite database in the test's own temp dir (schema
// applied, foreign_keys ON via repository.Open's DSN) and registers cleanup that
// closes the handle. Each call is fully isolated — its own file — so callers may run
// in parallel without contention. The caller builds the per-entity repositories it
// needs over the returned handle (nodes.New(db), users.New(db), …), exactly as the
// composition root does.
func OpenDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := repository.Open(t.Context(), filepath.Join(t.TempDir(), "subgen.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	return db
}

// SeededNode is the result of SeedNode: the created node id plus its two persisted
// inbounds (with the ids SQLite assigned), so a test can reference an inbound without
// re-reading the node.
type SeededNode struct {
	NodeID int64
	Smart  entity.Inbound // first inbound  (e.g. selective routing)
	Force  entity.Inbound // second inbound (e.g. a plain exit)
}

// SeedNode creates one node ("RU1") with two inbounds ("smart" :4433, "force" :8443)
// and returns the assigned ids. It is the common fixture for connection / routing / FK
// tests that need a node with referenceable inbounds. The node is created through the
// real nodes.Create + read back through nodes.Get, so the returned ids are the ones
// user_connections and the mihomo config must point at.
func SeedNode(t *testing.T, repo *nodes.Repository) SeededNode {
	t.Helper()

	id, err := repo.Create(t.Context(), entity.Node{
		Name:          "RU1",
		VPNHost:       "ru1.example",
		PanelBaseURL:  "https://ru1.example:2053",
		PanelBasePath: "/",
		Token:         "tok-ru1",
		Inbounds: []entity.Inbound{
			{Name: "smart", Port: 4433},
			{Name: "force", Port: 8443},
		},
	})
	require.NoError(t, err)

	return SeededNode{
		NodeID: id,
		Smart:  inboundByName(t, repo, id, "smart"),
		Force:  inboundByName(t, repo, id, "force"),
	}
}

// inboundByName reads back a node's inbound by its name and returns it (with the
// persisted id/port). Used to resolve seeded inbounds to the ids referenced by
// connections and the mihomo config.
func inboundByName(t *testing.T, repo *nodes.Repository, nodeID int64, name string) entity.Inbound {
	t.Helper()

	n, err := repo.Get(t.Context(), nodeID)
	require.NoError(t, err)

	for _, in := range n.Inbounds {
		if in.Name == name {
			return in
		}
	}

	require.FailNowf(t, "inbound not found", "node %d has no inbound %q", nodeID, name)

	return entity.Inbound{}
}

// SeedConfig creates the base subscription_configs row (kind=mihomo) and returns its
// id — the config_id the mihomo_* content tables must reference. Routing tests scope
// every read/write to it. Inserted directly (not via the configs repo) to keep dbtest
// free of the configs↔routing wiring.
func SeedConfig(t *testing.T, db *sql.DB) int64 {
	t.Helper()

	res, err := db.ExecContext(t.Context(),
		`INSERT INTO subscription_configs(user_id,kind,created_at) VALUES(NULL,?,0)`, entity.ConfigKindMihomo)
	require.NoError(t, err)

	id, err := res.LastInsertId()
	require.NoError(t, err)

	return id
}

// Ptr returns a pointer to v — for the *int64 ids inside mihomo.PolicyRef (InboundID /
// GroupID), which are nil for built-in policies and set for inbound/group refs.
func Ptr[T any](v T) *T { return &v }

// RuleToInbound builds a single MATCH rule whose target is the given inbound id — the
// minimal mihomo config that holds an FK to node_inbounds (for the RESTRICT tests).
func RuleToInbound(inboundID int64) mihomo.RoutingRule {
	return mihomo.RoutingRule{
		Type:   mihomo.RuleMatch,
		Target: mihomo.PolicyRef{Kind: mihomo.PolicyInbound, InboundID: Ptr(inboundID)},
	}
}

// GroupWithInbound builds a select proxy-group with one member of kind inbound — the
// other way the mihomo config holds an FK to node_inbounds (via a group member).
func GroupWithInbound(name string, inboundID int64) mihomo.ProxyGroup {
	return mihomo.ProxyGroup{
		Name: name, Type: mihomo.GroupSelect,
		Members: []mihomo.PolicyRef{{Kind: mihomo.PolicyInbound, InboundID: Ptr(inboundID)}},
	}
}
