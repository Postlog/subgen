//go:build integration

package users_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/repository/dbtest"
	"github.com/postlog/subgen/internal/repository/nodes"
	"github.com/postlog/subgen/internal/repository/users"
)

// TestRepository_ListPage covers the SQL-level filter/paging: name substring (case-
// insensitive, LIKE-metachar safe), inbound OR-filter, their AND combination, paging
// windows + total, and connection hydration scoped to the page.
func TestRepository_ListPage(t *testing.T) {
	t.Parallel()
	db := dbtest.OpenDB(t)
	seed := dbtest.SeedNode(t, nodes.New(db))
	repo := users.New(db)

	// Seed (inserted out of name order). Ordered by name (BINARY): a_b, amy, axb, bob, zoe.
	// "a_b"/"axb" exist to prove the '_' wildcard is escaped to a literal.
	mk := func(name, sub string, ins ...entity.Inbound) {
		conns := make([]entity.Connection, len(ins))
		for i, in := range ins {
			conns[i] = entity.Connection{InboundID: in.ID}
		}

		require.NoError(t, repo.Create(t.Context(), &entity.User{Name: name, SubID: sub, Connections: conns}))
	}

	mk("zoe", "s-zoe", seed.Smart, seed.Force)
	mk("amy", "s-amy", seed.Smart)
	mk("bob", "s-bob", seed.Force)
	mk("a_b", "s-a_b", seed.Smart)
	mk("axb", "s-axb", seed.Force)

	tt := []struct {
		name      string
		params    entity.UserListParams
		wantNames []string
		wantTotal int64
		wantConns map[string]int // optional: name -> #connections
	}{
		{name: "all.page1", params: entity.UserListParams{Limit: 2, Offset: 0}, wantNames: []string{"a_b", "amy"}, wantTotal: 5},
		{name: "all.page2", params: entity.UserListParams{Limit: 2, Offset: 2}, wantNames: []string{"axb", "bob"}, wantTotal: 5},
		{name: "all.page3", params: entity.UserListParams{Limit: 2, Offset: 4}, wantNames: []string{"zoe"}, wantTotal: 5, wantConns: map[string]int{"zoe": 2}},
		{name: "name.substring", params: entity.UserListParams{NameQuery: "o", Limit: 10}, wantNames: []string{"bob", "zoe"}, wantTotal: 2},
		{name: "name.case_insensitive", params: entity.UserListParams{NameQuery: "AM", Limit: 10}, wantNames: []string{"amy"}, wantTotal: 1},
		{name: "name.escapes_underscore", params: entity.UserListParams{NameQuery: "a_b", Limit: 10}, wantNames: []string{"a_b"}, wantTotal: 1},
		{name: "inbound.smart_or", params: entity.UserListParams{InboundIDs: []int64{seed.Smart.ID}, Limit: 10}, wantNames: []string{"a_b", "amy", "zoe"}, wantTotal: 3},
		{name: "inbound.both_or", params: entity.UserListParams{InboundIDs: []int64{seed.Smart.ID, seed.Force.ID}, Limit: 10}, wantNames: []string{"a_b", "amy", "axb", "bob", "zoe"}, wantTotal: 5, wantConns: map[string]int{"a_b": 1, "amy": 1, "axb": 1, "bob": 1, "zoe": 2}},
		{name: "name_and_inbound", params: entity.UserListParams{NameQuery: "a", InboundIDs: []int64{seed.Force.ID}, Limit: 10}, wantNames: []string{"axb"}, wantTotal: 1},
		{name: "no_match", params: entity.UserListParams{NameQuery: "nobody", Limit: 10}, wantNames: nil, wantTotal: 0},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			page, err := repo.ListPage(t.Context(), tc.params)
			require.NoError(t, err)
			assert.Equal(t, tc.wantTotal, page.Total)
			assert.Equal(t, tc.wantNames, names(page.Users))

			for name, n := range tc.wantConns {
				assert.Len(t, userByName(page.Users, name).Connections, n, "connections of %q", name)
			}
		})
	}
}

func names(us []entity.User) []string {
	if len(us) == 0 {
		return nil
	}

	out := make([]string, len(us))
	for i := range us {
		out[i] = us[i].Name
	}

	return out
}

func userByName(us []entity.User, name string) entity.User {
	for _, u := range us {
		if u.Name == name {
			return u
		}
	}

	return entity.User{}
}
