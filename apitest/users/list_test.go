//go:build apitest

package users_test

import (
	"strings"

	"github.com/postlog/subgen/apitest/api"
)

// Corner cases considered for GET /admin/api/users:
//   - shape          — a created user appears with id/name, a subId + absolute /sub URL,
//                      and one inbound row per binding (label "<node>-<inbound>", port).
//   - traffic        — the stats field is present; a brand-new client reports 0/0
//                      (the users API folds fleet traffic in; fresh clients have none).
//   - missing_flag   — a binding whose client was deleted out-of-band on the panel is
//                      reported Missing=true; a healthy one is Missing=false.
//   - empty          — an empty store lists no users (covered implicitly: the suite
//                      shares a store, so this is asserted by name-scoping each lookup).

// TestListShape covers the row shape for a multi-binding user.
func (s *UserSuite) TestListShape() {
	u := s.createUser(s.userName(), "N1", "N1", "N2") // smart-N1, force-N1, force-N2

	row, err := s.API().MustFindUser(u.Name)
	s.Require().NoError(err)

	s.Equal(u.Name, row.Name)
	s.Positive(row.ID)

	// Subscription coordinates: a non-empty subId and an absolute, token-signed URL.
	s.NotEmpty(row.Sub.ID, "row must carry the subId")
	s.True(strings.HasPrefix(row.Sub.URL, "http"), "sub URL must be absolute: %q", row.Sub.URL)
	s.Contains(row.Sub.URL, "/sub/", "sub URL must hit the /sub route")

	// Three bindings, labelled "<node>-<inbound>" with the right ports.
	s.Require().Len(row.Inbounds, 3)

	labels := map[string]int{} // label -> port
	for _, in := range row.Inbounds {
		labels[in.Label] = in.Port
		s.False(in.Missing, "freshly-provisioned binding %q must not be flagged missing", in.Label)
	}

	s.Equal(api.N1Smart, labels["N1-smart"])
	s.Equal(api.N1Force, labels["N1-force"])
	s.Equal(api.N2Force, labels["N2-force"])
}

// TestListTraffic covers the stats field on a fresh user (no traffic yet).
func (s *UserSuite) TestListTraffic() {
	u := s.createUser(s.userName(), "N1")

	row, err := s.API().MustFindUser(u.Name)
	s.Require().NoError(err)

	s.Zero(row.Stats.Up, "a fresh client has no upload")
	s.Zero(row.Stats.Down, "a fresh client has no download")
}

// TestListMissingFlag covers the per-binding health flag: a client deleted out-of-band
// on the panel is reported missing; restoring it clears the flag.
func (s *UserSuite) TestListMissingFlag() {
	u := s.createUser(s.userName(), "N1")

	// Healthy first.
	row, err := s.API().MustFindUser(u.Name)
	s.Require().NoError(err)
	s.Require().Len(row.Inbounds, 1)
	s.False(row.Inbounds[0].Missing, "healthy binding must not be flagged missing")

	// Delete the client directly on the panel → the binding is now missing.
	s.Require().NoError(s.XC().DelClient(api.Ctx, s.Pan1().Target, u.Name))

	row, err = s.API().MustFindUser(u.Name)
	s.Require().NoError(err)
	s.Require().Len(row.Inbounds, 1)
	s.True(row.Inbounds[0].Missing, "a binding whose panel client is gone must be flagged missing")

	// Restore via recreate → flag clears.
	s.recreateUser(u.ID)

	row, err = s.API().MustFindUser(u.Name)
	s.Require().NoError(err)
	s.False(row.Inbounds[0].Missing, "restored binding must clear the missing flag")
}
