//go:build apitest

package users_test

import "github.com/postlog/subgen/apitest/api"

// Corner cases considered for POST /admin/api/users/recreate:
//   - happy.restore_drift   — a client deleted out-of-band on the panel is restored.
//   - idempotency.run_twice — a second recreate leaves exactly one client (no churn error).
//   - err.unknown_id        — recreating a non-existent user → {ok:false}.

// TestRecreate covers drift recovery: if a client vanishes from the panel, recreate
// restores it; a second run is idempotent.
func (s *UserSuite) TestRecreate() {
	u := s.createUser(s.userName(), "N1")
	s.RequireClient(s.Pan1(), api.N1Smart, u.Name)

	// Simulate drift: delete the client directly on the panel.
	s.Require().NoError(s.XC().DelClient(api.Ctx, s.Pan1().Target, u.Name))
	s.RequireNoClient(s.Pan1(), api.N1Smart, u.Name)

	// Recreate restores it.
	res, err := s.API().RecreateUser(u.ID)
	s.Require().NoError(err)
	s.Require().True(res.OK, "recreate: %s", res.Message())
	s.Equal(msgRecreated, res.Msg)
	s.RequireClient(s.Pan1(), api.N1Smart, u.Name)

	// Idempotent: still exactly one client afterwards.
	s.recreateUser(u.ID)
	s.RequireClient(s.Pan1(), api.N1Smart, u.Name)
}

// TestRecreateUnknownID covers recreating a never-existed id.
func (s *UserSuite) TestRecreateUnknownID() {
	res, err := s.API().RecreateUser(99999999)
	s.Require().NoError(err)
	s.False(res.OK, "recreating a non-existent user must fail")
	s.NotEmpty(res.Err)
}
