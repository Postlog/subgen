//go:build integration

package nodes_test

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/repository/dbtest"
	"github.com/postlog/subgen/internal/repository/nodes"
	"github.com/postlog/subgen/internal/repository/users"
)

func TestRepository_Update(t *testing.T) {
	tt := []struct {
		name string

		// arrange optionally seeds rows (a colliding node, a referencing connection) over
		// the shared db handle before the Update. mutate receives the seeded node (with
		// persisted inbound ids) and returns the node value + setToken to pass to Update;
		// assertAfter inspects the result.
		arrange     func(t *testing.T, db *sql.DB, seed dbtest.SeededNode)
		mutate      func(seed dbtest.SeededNode) (entity.Node, bool)
		assertAfter func(t *testing.T, repo *nodes.Repository, seed dbtest.SeededNode)

		err     error
		wantErr bool // for the inline "token is empty" (no sentinel)
	}{
		{
			name: "success.rename_keeps_token",
			mutate: func(seed dbtest.SeededNode) (entity.Node, bool) {
				return entity.Node{
					Name: "RU1-renamed", VPNHost: "ru1.example",
					PanelBaseURL: "https://ru1:2053", PanelBasePath: "/",
				}, false // setToken=false
			},
			assertAfter: func(t *testing.T, repo *nodes.Repository, seed dbtest.SeededNode) {
				got, err := repo.Get(t.Context(), seed.NodeID)
				require.NoError(t, err)
				assert.Equal(t, "RU1-renamed", got.Name)
				assert.Equal(t, "tok-ru1", got.Token) // COALESCE kept it
			},
		},
		{
			name: "success.replace_token",
			mutate: func(seed dbtest.SeededNode) (entity.Node, bool) {
				return entity.Node{Name: "RU1", VPNHost: "ru1.example", Token: "tok-new"}, true
			},
			assertAfter: func(t *testing.T, repo *nodes.Repository, seed dbtest.SeededNode) {
				got, err := repo.Get(t.Context(), seed.NodeID)
				require.NoError(t, err)
				assert.Equal(t, "tok-new", got.Token)
			},
		},
		{
			name: "success.inbound_diff_keeps_adds_deletes",
			mutate: func(seed dbtest.SeededNode) (entity.Node, bool) {
				// Keep "smart" by its stable id (renamed + ported), drop "force" (absent),
				// add a brand new inbound (id==0).
				return entity.Node{
					Name: "RU1", VPNHost: "ru1.example",
					Inbounds: []entity.Inbound{
						{ID: seed.Smart.ID, Name: "smart2", Port: 4434},
						{ID: 0, Name: "fresh", Port: 9999},
					},
				}, false
			},
			assertAfter: func(t *testing.T, repo *nodes.Repository, seed dbtest.SeededNode) {
				got, err := repo.Get(t.Context(), seed.NodeID)
				require.NoError(t, err)

				byName := map[string]entity.Inbound{}
				for _, in := range got.Inbounds {
					byName[in.Name] = in
				}
				require.Len(t, got.Inbounds, 2)

				// "smart" kept its id across the rename/port change.
				assert.Equal(t, seed.Smart.ID, byName["smart2"].ID)
				assert.Equal(t, 4434, byName["smart2"].Port)
				// "force" is gone; "fresh" was added with a new id.
				_, hasForce := byName["force"]
				assert.False(t, hasForce)
				assert.NotZero(t, byName["fresh"].ID)
				assert.NotEqual(t, seed.Smart.ID, byName["fresh"].ID)
			},
		},
		{
			name: "error.empty_token_when_set",
			mutate: func(seed dbtest.SeededNode) (entity.Node, bool) {
				return entity.Node{Name: "RU1", Token: ""}, true // setToken=true but empty
			},
			wantErr: true,
		},
		{
			name: "error.duplicate_node_name",
			// A second node "NL2" exists; renaming RU1 onto it hits the UNIQUE(name).
			arrange: func(t *testing.T, db *sql.DB, seed dbtest.SeededNode) {
				_, err := nodes.New(db).Create(t.Context(), entity.Node{Name: "NL2", VPNHost: "nl2", Token: "t"})
				require.NoError(t, err)
			},
			mutate: func(seed dbtest.SeededNode) (entity.Node, bool) {
				return entity.Node{Name: "NL2", VPNHost: "ru1.example"}, false
			},
			err: entity.ErrNodeNameTaken,
		},
		{
			name: "error.remove_referenced_inbound",
			// "force" still has a user connection; dropping it from the submission (keeping
			// only "smart") makes the cascade DELETE hit the user_connections FK — surfaced
			// as the clean sentinel, not a raw FK error.
			arrange: func(t *testing.T, db *sql.DB, seed dbtest.SeededNode) {
				u := &entity.User{
					Name: "bob", SubID: "sub-bob",
					Connections: []entity.Connection{{InboundID: seed.Force.ID}},
				}
				require.NoError(t, users.New(db).Create(t.Context(), u))
			},
			mutate: func(seed dbtest.SeededNode) (entity.Node, bool) {
				return entity.Node{
					Name: "RU1", VPNHost: "ru1.example",
					Inbounds: []entity.Inbound{{ID: seed.Smart.ID, Name: "smart", Port: seed.Smart.Port}},
				}, false
			},
			err: entity.ErrInboundReferenced,
		},
		{
			name: "error.duplicate_inbound_port",
			mutate: func(seed dbtest.SeededNode) (entity.Node, bool) {
				// Update "smart" onto "force"'s port — collides with the kept "force".
				return entity.Node{
					Name: "RU1",
					Inbounds: []entity.Inbound{
						{ID: seed.Smart.ID, Name: "smart", Port: seed.Force.Port},
						{ID: seed.Force.ID, Name: "force", Port: seed.Force.Port},
					},
				}, false
			},
			err: entity.ErrInboundDuplicate,
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

			n, setToken := tc.mutate(seed)
			err := repo.Update(t.Context(), seed.NodeID, n, setToken)

			if tc.wantErr {
				require.ErrorContains(t, err, "token is empty")
				return
			}

			require.ErrorIs(t, err, tc.err)

			if tc.assertAfter != nil {
				tc.assertAfter(t, repo, seed)
			}
		})
	}
}
