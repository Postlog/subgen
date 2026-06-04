//go:build apitest

// Package auth_test drives subgen's admin shell endpoints over the real HTTP API:
// POST/GET /admin/login, GET /admin/logout, the session gate on /admin/api/*, and the
// static + SPA-shell GETs. None of these touch a 3x-ui panel, so the suite boots a
// server WITHOUT registering any nodes and does NOT gate on the panels being
// configured — it runs in plain CI. One Test* per endpoint/scenario; corner cases are
// dotted subtests.
package auth_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/postlog/subgen/apitest/api"
)

// AuthSuite boots a subgen server (admin panel enabled) and drives the shell endpoints.
// It deliberately does NOT embed api.Base (which would register nodes against the
// panels) — auth needs no panel — so it owns a bare server + two SDK clients: one it
// logs in, one it leaves anonymous.
type AuthSuite struct {
	suite.Suite

	server *api.Server
	authed *api.Client // logged-in client
	anon   *api.Client // never logs in
}

func TestAuthSuite(t *testing.T) {
	// No SkipUnlessConfigured: this suite needs no panels, so it runs everywhere.
	suite.Run(t, new(AuthSuite))
}

func (s *AuthSuite) SetupSuite() {
	s.server = api.StartServer(s.T())

	s.authed = api.New(s.server.BaseURL())
	res, err := s.authed.Login(api.AdminUser, api.AdminPass)
	s.Require().NoError(err)
	s.Require().True(res.OK, "admin login must succeed: %s", res.Message())

	s.anon = api.New(s.server.BaseURL())
}

// fresh returns a brand-new anonymous client against the same server (so a login-cookie
// capture in one case doesn't leak into another).
func (s *AuthSuite) fresh() *api.Client { return api.New(s.server.BaseURL()) }
