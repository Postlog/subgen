//go:build integration

package nodes_test

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/mihomo"
	"github.com/postlog/subgen/internal/repository/dbtest"
	"github.com/postlog/subgen/internal/repository/nodes"
	"github.com/postlog/subgen/internal/repository/routing"
	"github.com/postlog/subgen/internal/repository/users"
)

// Delete is RESTRICTed by every reference to its inbounds — a user_connection, a
// mihomo proxy-group member, or a mihomo routing rule — so the cross-entity cases
// seed those references through the sibling users/routing repositories over the same
// db handle (the FKs are real because it is one SQLite file).
func TestRepository_Delete(t *testing.T) {
	tt := []struct {
		name string

		// arrange sets up references against the seeded node that should block (or, when
		// nil, allow) its deletion. db is the shared handle so it can build the sibling
		// repositories that own the referencing rows.
		arrange func(t *testing.T, db *sql.DB, seed dbtest.SeededNode)
		wantErr bool
	}{
		{
			name: "success.cascades_inbounds",
			// No references: delete succeeds and cascades node_inbounds.
		},
		{
			name: "error.referenced_by_user_connection",
			arrange: func(t *testing.T, db *sql.DB, seed dbtest.SeededNode) {
				u := &entity.User{
					Name: "alice", SubID: "sub-alice",
					Connections: []entity.Connection{{InboundID: seed.Smart.ID}},
				}
				require.NoError(t, users.New(db).Create(t.Context(), u))
			},
			wantErr: true, // FK from user_connections RESTRICTs the cascade delete
		},
		{
			name: "error.referenced_by_mihomo_member",
			arrange: func(t *testing.T, db *sql.DB, seed dbtest.SeededNode) {
				// A proxy-group member of kind inbound holds an FK to node_inbounds.
				cfg := dbtest.SeedConfig(t, db)
				require.NoError(t, routing.New(db).SaveMihomoConfig(t.Context(), cfg,
					nil, []mihomo.ProxyGroup{dbtest.GroupWithInbound("sel", seed.Smart.ID)}, nil, "", mihomo.Profile{}))
			},
			wantErr: true, // FK from mihomo_proxy_group_members.inbound_id RESTRICTs it
		},
		{
			name: "error.referenced_by_mihomo_rule",
			arrange: func(t *testing.T, db *sql.DB, seed dbtest.SeededNode) {
				// A routing rule targeting an inbound holds an FK to node_inbounds.
				cfg := dbtest.SeedConfig(t, db)
				require.NoError(t, routing.New(db).SaveMihomoConfig(t.Context(), cfg,
					[]mihomo.RoutingRule{dbtest.RuleToInbound(seed.Smart.ID)}, nil, nil, "", mihomo.Profile{}))
			},
			wantErr: true, // FK from mihomo_routing_rules.inbound_id RESTRICTs it
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

			err := repo.Delete(t.Context(), seed.NodeID)

			if tc.wantErr {
				// A still-referenced inbound surfaces as the clean sentinel, not a raw FK error.
				require.ErrorIs(t, err, entity.ErrInboundReferenced)

				// The node (and its inbounds) survive a blocked delete.
				got, getErr := repo.Get(t.Context(), seed.NodeID)
				require.NoError(t, getErr)
				assert.Equal(t, seed.NodeID, got.ID)
				return
			}

			require.NoError(t, err)

			_, getErr := repo.Get(t.Context(), seed.NodeID)
			assert.ErrorIs(t, getErr, sql.ErrNoRows)
		})
	}
}

// TestRepository_Delete_unknownID checks that deleting an id that isn't there removes
// nothing and reports entity.ErrNodeNotFound (from rows-affected, not a pre-check).
func TestRepository_Delete_unknownID(t *testing.T) {
	t.Parallel()

	repo := nodes.New(dbtest.OpenDB(t))

	err := repo.Delete(t.Context(), 99999999)
	require.ErrorIs(t, err, entity.ErrNodeNotFound)
}
