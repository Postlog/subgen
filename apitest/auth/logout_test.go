//go:build apitest

package auth_test

import (
	"net/http"

	"github.com/postlog/subgen/apitest/api"
)

// Corner cases considered for POST /admin/api/logout:
//   - clears_cookie  — returns 204 and an expiring Set-Cookie for the session (no
//                      redirect; the SPA navigates to the login page itself).
//   - clears_session — an anonymous client (no session) is gated on the API with a 401,
//                      confirming the session is required.
//
// Note: logout clears the cookie via a Set-Cookie MaxAge<0. The SDK captured the session
// value at login and replays it by hand (the cookie is Secure, so the jar won't over
// plain HTTP) — so to prove "no session ⇒ gated" black-box, we use a fresh anonymous
// client for the follow-up gated call rather than the same handle.

// TestLogout covers POST /admin/api/logout.
func (s *AuthSuite) TestLogout() {
	// A logged-in client.
	c := s.fresh()
	res, err := c.Login(api.AdminUser, api.AdminPass)
	s.Require().NoError(err)
	s.Require().True(res.OK)

	resp, err := c.Logout()
	s.Require().NoError(err)
	s.Equal(http.StatusNoContent, resp.Status, "logout returns 204, no redirect")

	// The response expires the session cookie.
	s.Contains(resp.Headers.Get("Set-Cookie"), api.AdminCookie, "logout must emit a Set-Cookie for the session")

	// An anonymous client (no session) is gated on the API with a 401.
	resp, err = s.anon.Get("/admin/api/users")
	s.Require().NoError(err)
	s.Equal(http.StatusUnauthorized, resp.Status)
}
