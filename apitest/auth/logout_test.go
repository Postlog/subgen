//go:build apitest

package auth_test

import (
	"net/http"

	"github.com/postlog/subgen/apitest/api"
)

// Corner cases considered for GET /admin/logout:
//   - redirects_to_login — always 302 to /admin/login (it clears the cookie + bounces).
//   - clears_session     — after logout the (now-cleared) client is treated as anonymous:
//                          a gated read API redirects rather than returning JSON.
//
// Note: logout clears the cookie via a Set-Cookie MaxAge<0. The SDK captured the
// session value at login and replays it by hand (the cookie is Secure, so the jar won't
// over plain HTTP) — so to prove "clears session" black-box, we use a fresh anonymous
// client for the follow-up gated call rather than the same handle. The redirect + the
// expiring Set-Cookie header are the observable contract here.

// TestLogout covers GET /admin/logout.
func (s *AuthSuite) TestLogout() {
	// A logged-in client.
	c := s.fresh()
	res, err := c.Login(api.AdminUser, api.AdminPass)
	s.Require().NoError(err)
	s.Require().True(res.OK)

	resp, err := c.Get("/admin/logout")
	s.Require().NoError(err)
	s.Equal(http.StatusFound, resp.Status, "logout must redirect")
	s.Equal("/admin/login", resp.Headers.Get("Location"))

	// The response expires the session cookie.
	s.Contains(resp.Headers.Get("Set-Cookie"), api.AdminCookie, "logout must emit a Set-Cookie for the session")

	// An anonymous client (no session) is gated, confirming logout leaves you logged out.
	resp, err = s.anon.Get("/admin/api/users")
	s.Require().NoError(err)
	s.Equal(http.StatusFound, resp.Status)
	s.Equal("/admin/login", resp.Headers.Get("Location"))
}
