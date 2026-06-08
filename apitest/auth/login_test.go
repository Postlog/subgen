//go:build apitest

package auth_test

import (
	"net/http"
	"strings"

	"github.com/postlog/subgen/apitest/api"
)

// msgBadCreds is the exact rejection text the login handler returns for wrong creds.
const msgBadCreds = "Неверный логин или пароль"

// Corner cases considered for POST /admin/api/login (idiomatic contract: 200 {message}
// on success, 4xx {errMessage} otherwise):
//   - ok             — right user+password → 200 "ok" + a Secure session cookie.
//   - wrong_user     — bad user, right password → 401 + the friendly creds text.
//   - wrong_password — right user, bad password → 401 + the friendly creds text.
//   - empty_fields   — "" / "" → 400 generic (schema minLength:1 trips before the handler).
//   - missing_fields — {} (no keys at all) → 400 generic (required fields missing).
//   - malformed_json — non-JSON body → 400 generic (request decode failure).
//
// And for GET /admin/login (the page):
//   - get.unauthed_login_page — anonymous GET serves the login HTML (200).
//   - get.authed_redirect     — an authenticated GET redirects (302) away from login.

// TestLoginPost covers every POST /admin/api/login outcome.
func (s *AuthSuite) TestLoginPost() {
	s.Run("ok", func() {
		c := s.fresh()
		res, err := c.Login(api.AdminUser, api.AdminPass)
		s.Require().NoError(err)
		s.Equal(http.StatusOK, res.Status)
		s.True(res.OK, "valid creds must be accepted")
		s.Equal("ok", res.Msg)
		s.True(c.Authed(), "a successful login must capture the session cookie")
	})

	s.Run("wrong_user", func() {
		res, err := s.fresh().Login("not-the-admin", api.AdminPass)
		s.Require().NoError(err)
		s.Equal(http.StatusUnauthorized, res.Status, "wrong creds are 401")
		s.False(res.OK)
		s.Equal(msgBadCreds, res.Err)
	})

	s.Run("wrong_password", func() {
		res, err := s.fresh().Login(api.AdminUser, "wrong-password")
		s.Require().NoError(err)
		s.Equal(http.StatusUnauthorized, res.Status)
		s.False(res.OK)
		s.Equal(msgBadCreds, res.Err)
	})

	s.Run("empty_fields", func() {
		// "" / "" trips the schema's minLength:1 before the handler runs → generic 400.
		res, err := s.fresh().Login("", "")
		s.Require().NoError(err)
		s.Equal(http.StatusBadRequest, res.Status, "empty creds fail schema validation")
		s.False(res.OK)
		s.Equal(api.MsgBadRequest, res.Err)
	})

	s.Run("missing_fields", func() {
		// A body with no user/password keys at all → required-field validation → 400.
		res, status, err := s.fresh().LoginRaw([]byte(`{}`))
		s.Require().NoError(err)
		s.Equal(http.StatusBadRequest, status)
		s.False(res.OK)
		s.Equal(api.MsgBadRequest, res.Err)
	})

	s.Run("malformed_json", func() {
		res, status, err := s.fresh().LoginRaw([]byte("{not json"))
		s.Require().NoError(err)
		s.Equal(http.StatusBadRequest, status, "a malformed body is a 400")
		s.False(res.OK)
		s.Equal(api.MsgBadRequest, res.Err)
	})
}

// TestLoginGet covers GET /admin/login: the page for anonymous users, a redirect for
// authenticated ones.
func (s *AuthSuite) TestLoginGet() {
	s.Run("unauthed_login_page", func() {
		resp, err := s.anon.Get("/admin/login")
		s.Require().NoError(err)
		s.Equal(http.StatusOK, resp.Status)
		s.Contains(resp.Headers.Get("Content-Type"), "text/html")
		// The served asset is login.html — it carries the login form.
		s.True(strings.Contains(strings.ToLower(string(resp.Body)), "<form") ||
			strings.Contains(strings.ToLower(string(resp.Body)), "password"),
			"GET /admin/login must serve the login page")
	})

	s.Run("authed_redirect", func() {
		resp, err := s.authed.Get("/admin/login")
		s.Require().NoError(err)
		s.Equal(http.StatusFound, resp.Status, "an authenticated GET /admin/login must redirect")
		s.Contains(resp.Headers.Get("Location"), "/admin", "redirect must target the admin app")
	})
}
