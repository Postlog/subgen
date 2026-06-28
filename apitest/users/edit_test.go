//go:build apitest

package users_test

import (
	"net/http"

	"github.com/postlog/subgen/apitest/api"
	userEditHandler "github.com/postlog/subgen/internal/handlers/user_edit"
)

// Corner cases considered for POST /admin/api/users/edit:
//   - reconcile.add_inbound      — add a force on a new node; existing node's client uuid
//                                  stays stable; both share the subId.
//   - reconcile.remove_inbound   — drop the added force; only that panel is cleared.
//   - reconcile.same_node_pair   — smart+force on one node toggling force keeps one uuid.
//   - reconcile.type_swap        — smart→force on the same node moves the client, keeps uuid.
//   - reconcile.cross_swap       — smart=N1,force=N2 → smart=N2,force=N1 in one edit.
//   - reconcile.smart_move       — move the only (smart) inbound to another node.
//   - noop                       — identical selection must NOT churn the panel (uuid stable).
//   - err.no_connection          — edit with an absent inbound list (null) → generic 400 (kept `required`), no change.
//   - err.unknown_user           — id with no user row → failure, technical error surfaced.
//   - err.unknown_inbound        — selection includes a bad inbound id → "inbound not found".

// TestEditReconcile covers the add/remove + same-node + swap reconciliations, asserting
// the panel ends in the expected state and uuids are preserved where they should be.
func (s *UserSuite) TestEditReconcile() {
	s.Run("add_then_remove_cross_node", func() {
		u := s.createUser(s.userName(), "N1") // smart on N1 only
		uuid1 := s.RequireClient(s.Pan1(), api.N1Smart, u.Name)
		s.RequireNoClient(s.Pan2(), api.N2Force, u.Name)

		// Add force on N2 — N1 untouched, both share the subId.
		s.editUser(u.ID, "N1", "N2")
		s.Equal(uuid1, s.RequireClient(s.Pan1(), api.N1Smart, u.Name), "adding a remote node must not churn N1")
		uuid2 := s.RequireClient(s.Pan2(), api.N2Force, u.Name)

		px := s.subProxies(u)
		s.Equal(uuid1, px["N1-smart"])
		s.Equal(uuid2, px["N2-force"])

		// Remove the N2 force — only N2 cleared.
		s.editUser(u.ID, "N1")
		s.RequireNoClient(s.Pan2(), api.N2Force, u.Name)
		s.Equal(uuid1, s.RequireClient(s.Pan1(), api.N1Smart, u.Name))
	})

	s.Run("same_node_pair_toggle", func() {
		u := s.createUser(s.userName(), "N1", "N1") // smart + force on N1
		uuidSmart := s.RequireClient(s.Pan1(), api.N1Smart, u.Name)
		s.Equal(uuidSmart, s.RequireClient(s.Pan1(), api.N1Force, u.Name))

		// Drop force → only smart, same uuid.
		s.editUser(u.ID, "N1")
		s.Equal(uuidSmart, s.RequireClient(s.Pan1(), api.N1Smart, u.Name), "uuid preserved when dropping force")
		s.RequireNoClient(s.Pan1(), api.N1Force, u.Name)

		// Add force back → both inbounds again, one client.
		s.editUser(u.ID, "N1", "N1")
		s.Equal(s.ClientUUID(s.Pan1(), api.N1Smart, u.Name), s.RequireClient(s.Pan1(), api.N1Force, u.Name))
	})

	s.Run("type_swap_same_node", func() {
		u := s.createUser(s.userName(), "N1") // smart on N1
		uuid := s.RequireClient(s.Pan1(), api.N1Smart, u.Name)

		s.editUser(u.ID, "", "N1") // force on N1, no smart
		s.RequireNoClient(s.Pan1(), api.N1Smart, u.Name)
		s.Equal(uuid, s.RequireClient(s.Pan1(), api.N1Force, u.Name), "uuid preserved across a same-node type swap")
	})

	s.Run("cross_swap", func() {
		u := s.createUser(s.userName(), "N1", "N2") // smart=N1, force=N2
		s.RequireClient(s.Pan1(), api.N1Smart, u.Name)
		s.RequireClient(s.Pan2(), api.N2Force, u.Name)

		s.editUser(u.ID, "N2", "N1") // smart=N2, force=N1
		s.RequireNoClient(s.Pan1(), api.N1Smart, u.Name)
		s.RequireClient(s.Pan1(), api.N1Force, u.Name)
		s.RequireNoClient(s.Pan2(), api.N2Force, u.Name)
		s.RequireClient(s.Pan2(), api.N2Smart, u.Name)
	})

	s.Run("smart_move", func() {
		u := s.createUser(s.userName(), "N1")
		s.RequireClient(s.Pan1(), api.N1Smart, u.Name)

		s.editUser(u.ID, "N2") // smart now on N2
		s.RequireNoClient(s.Pan1(), api.N1Smart, u.Name)
		s.RequireClient(s.Pan2(), api.N2Smart, u.Name)
	})
}

// TestEditNoop covers the idempotent edit: an identical selection must not delete+re-add
// the client (the uuid stays the same).
func (s *UserSuite) TestEditNoop() {
	u := s.createUser(s.userName(), "N1", "N1")
	before := s.RequireClient(s.Pan1(), api.N1Smart, u.Name)

	s.editUser(u.ID, "N1", "N1") // identical selection

	s.Equal(before, s.RequireClient(s.Pan1(), api.N1Smart, u.Name), "no-op edit must keep the uuid")
	s.Equal(before, s.RequireClient(s.Pan1(), api.N1Force, u.Name))
}

// TestEditValidation covers the rejected edits.
func (s *UserSuite) TestEditValidation() {
	u := s.createUser(s.userName(), "N1")

	s.Run("no_connection", func() {
		// An absent inbound list (nil → null) is rejected as a generic 400 by the kept
		// `required` (value constraints like minItems live in the service, not the schema, but
		// `required` stays); the binding is unchanged.
		res, err := s.API().EditUser(u.ID, nil)
		s.Require().NoError(err)
		s.Equal(http.StatusBadRequest, res.Status)
		s.False(res.OK)
		s.Equal(api.MsgBadRequest, res.Err)
		// Unchanged on the panel.
		s.RequireClient(s.Pan1(), api.N1Smart, u.Name)
	})

	s.Run("unknown_inbound", func() {
		res, err := s.API().EditUser(u.ID, []int64{999999})
		s.Require().NoError(err)
		s.False(res.OK)
		s.Equal(userEditHandler.MsgInboundNotFound, res.Err)
	})

	s.Run("unknown_user", func() {
		// A real inbound id but a non-existent user id → users.Get fails; the error is
		// technical (not a friendly sentinel), so just assert failure with a message.
		res, err := s.API().EditUser(99999999, []int64{s.InboundID("N1", "smart")})
		s.Require().NoError(err)
		s.False(res.OK, "editing a non-existent user must fail")
		s.NotEmpty(res.Err)
	})
}
