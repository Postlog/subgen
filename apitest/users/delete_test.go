//go:build apitest

package users_test

import (
	"github.com/postlog/subgen/apitest/api"
	userDeleteHandler "github.com/postlog/subgen/internal/handlers/user_delete"
)

// Corner cases considered for POST /admin/api/users/delete:
//   - happy.multi_panel       — removes the user's clients from every panel it's on,
//                               leaving a neighbour user's client untouched.
//   - idempotency.second_delete — deleting again returns {ok:false} (user row gone), and
//                               the panel stays clean (no client resurrected).
//   - err.unknown_id          — deleting a non-existent id → {ok:false}, technical error.

// TestDeleteMultiPanel covers the happy path: delete clears every panel the user is on,
// neighbours survive.
func (s *UserSuite) TestDeleteMultiPanel() {
	neighbor := s.createUser(s.userName(), "N1") // must survive

	u := s.createUser(s.userName(), "N1", "N1", "N2") // smart=N1, force=[N1,N2]
	s.RequireClient(s.Pan1(), api.N1Smart, u.Name)
	s.RequireClient(s.Pan1(), api.N1Force, u.Name)
	s.RequireClient(s.Pan2(), api.N2Force, u.Name)

	res, err := s.API().DeleteUser(u.ID)
	s.Require().NoError(err)
	s.Require().True(res.OK, "delete: %s", res.Message())
	s.Equal(userDeleteHandler.MsgDeleted, res.Msg)

	s.RequireNoClient(s.Pan1(), api.N1Smart, u.Name)
	s.RequireNoClient(s.Pan1(), api.N1Force, u.Name)
	s.RequireNoClient(s.Pan2(), api.N2Force, u.Name)

	// The neighbour is untouched.
	s.RequireClient(s.Pan1(), api.N1Smart, neighbor.Name)

	// The user row is gone.
	gone, err := s.API().FindUser(u.Name)
	s.Require().NoError(err)
	s.Nil(gone)
}

// TestDeleteIdempotency covers a second delete of the same user.
func (s *UserSuite) TestDeleteIdempotency() {
	u := s.createUser(s.userName(), "N1")

	// First delete succeeds.
	res, err := s.API().DeleteUser(u.ID)
	s.Require().NoError(err)
	s.Require().True(res.OK)
	s.RequireNoClient(s.Pan1(), api.N1Smart, u.Name)

	// Second delete: the user row is gone → {ok:false}; the panel stays clean.
	res, err = s.API().DeleteUser(u.ID)
	s.Require().NoError(err)
	s.False(res.OK, "deleting an already-deleted user must report failure")
	s.RequireNoClient(s.Pan1(), api.N1Smart, u.Name)
}

// TestDeleteUnknownID covers deleting a never-existed id.
func (s *UserSuite) TestDeleteUnknownID() {
	res, err := s.API().DeleteUser(99999999)
	s.Require().NoError(err)
	s.False(res.OK, "deleting a non-existent user must fail")
	s.NotEmpty(res.Err)
}
