//go:build apitest

package users_test

import "github.com/postlog/subgen/apitest/api"

// Corner cases considered for a user's subscription content (GET /sub/{token} reached
// via the URL the users API reports — the user-facing view; the /sub endpoint mechanics
// like headers and unknown tokens are covered in the sub area):
//   - grouping.cross_node      — all of a user's clients (shared subId) appear in one
//                                profile with the expected proxy names and uuids.
//   - grouping.same_node_one_uuid — smart+force on one node render as two proxies that
//                                share one uuid (one underlying client).

// TestSubscriptionGrouping covers that the rendered subscription folds every binding of
// a user into one profile and each proxy's uuid matches the real panel client.
func (s *UserSuite) TestSubscriptionGrouping() {
	u := s.createUser(s.userName(), "N1", "N1", "N2") // smart-N1, force-N1, force-N2

	px := s.subProxies(u)
	for _, name := range []string{"N1-smart", "N1-force", "N2-force"} {
		s.Contains(px, name, "subscription must contain proxy %s", name)
	}

	s.Equal(s.ClientUUID(s.Pan1(), api.N1Smart, u.Name), px["N1-smart"])
	s.Equal(s.ClientUUID(s.Pan1(), api.N1Force, u.Name), px["N1-force"])
	s.Equal(s.ClientUUID(s.Pan2(), api.N2Force, u.Name), px["N2-force"])
	s.Equal(px["N1-smart"], px["N1-force"], "smart and force on one node are one client (one uuid)")
}
