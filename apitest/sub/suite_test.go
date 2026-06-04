//go:build apitest

// Package sub_test drives subgen's public (non-admin) endpoints over the real HTTP API:
// GET /healthz, GET /sub/{token} (the per-client mihomo subscription), and GET
// /rules/{file} (mirrored rule-provider files).
//
// Two suites split by what they need:
//
//   - SubSuite (UNGATED) — health, the /sub 404 paths (malformed/unknown token), and the
//     /rules paths (unknown file → 404, and a PRESENT mirrored file served from an
//     in-test upstream). None need a 3x-ui panel, so this runs in plain CI.
//   - SubPanelSuite (GATED) — a VALID /sub token: it provisions a real user (so it embeds
//     api.Base and gates on the panels) and asserts the rendered YAML + the filename /
//     profile / userinfo response headers.
//
// One Test* per endpoint/scenario; corner cases are dotted subtests.
package sub_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/postlog/subgen/apitest/api"
)

// SubSuite boots a bare server (no panels) for the health + 404 + mirror paths.
type SubSuite struct {
	suite.Suite

	server *api.Server
	api    *api.Client
}

func TestSubSuite(t *testing.T) {
	// No SkipUnlessConfigured: these paths need no docker panels.
	suite.Run(t, new(SubSuite))
}

func (s *SubSuite) SetupSuite() {
	s.server = api.StartServer(s.T())

	s.api = api.New(s.server.BaseURL())
	res, err := s.api.Login(api.AdminUser, api.AdminPass)
	s.Require().NoError(err)
	s.Require().True(res.OK, "admin login must succeed: %s", res.Message())
}

// SubPanelSuite covers a valid subscription end-to-end: it provisions a real user and
// fetches that user's /sub URL, so it embeds api.Base and is gated on the panels.
type SubPanelSuite struct{ api.Base }

func TestSubPanelSuite(t *testing.T) {
	api.SkipUnlessConfigured(t)
	suite.Run(t, new(SubPanelSuite))
}
