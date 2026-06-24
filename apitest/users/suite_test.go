//go:build apitest

// Package users_test drives subgen's user endpoints over the real HTTP API against the
// two docker 3x-ui panels, asserting both the response (2xx {message} / 4xx {errMessage},
// normalised into api.Result) AND the resulting panel client state. It embeds api.Base
// for the booted server + authenticated SDK +
// ground-truth probing. One Test* per endpoint/scenario lives in its own *_test.go;
// corner cases are dotted subtests. The suite runner gates on the panels being
// configured (it provisions onto them).
package users_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/postlog/subgen/apitest/api"
)

// UserSuite covers the admin user endpoints (create/edit/delete/recreate) and the
// users read API, driven over HTTP and verified against real 3x-ui panel state.
type UserSuite struct{ api.Base }

func TestUserSuite(t *testing.T) {
	api.SkipUnlessConfigured(t)
	suite.Run(t, new(UserSuite))
}

// ---- user lifecycle helpers (drive the SDK, auto-cleanup per scenario) -----
//
// They translate the ergonomic "smart node + force nodes" shorthand the scenarios use
// into the inbound-id selection the API takes, call the endpoint, assert {ok}, and (for
// create) read the user row back so the scenario has its id/subId/sub-URL.

// selection resolves a smart node + force nodes to the inbound-id list the user
// endpoints take. An empty smart is omitted; each force node contributes its force
// inbound.
func (s *UserSuite) selection(smart string, force []string) []int64 {
	var ids []int64

	if smart != "" {
		ids = append(ids, s.InboundID(smart, "smart"))
	}

	for _, f := range force {
		ids = append(ids, s.InboundID(f, "force"))
	}

	return ids
}

// createUser provisions a user over POST /admin/api/users/create (smart node +
// optional force nodes), asserts {ok}, reads the row back for its id/subId, and
// schedules its deletion at the end of the scenario.
func (s *UserSuite) createUser(name, smart string, force ...string) *api.User {
	res, err := s.API().CreateUser(name, s.selection(smart, force))
	s.Require().NoError(err)
	s.Require().True(res.OK, "CreateUser(%q): %s", name, res.Message())

	u, err := s.API().MustFindUser(name)
	s.Require().NoError(err)

	s.T().Cleanup(func() { _, _ = s.API().DeleteUser(u.ID) })

	return u
}

// editUser re-binds a user over POST /admin/api/users/edit and asserts {ok}.
func (s *UserSuite) editUser(id int64, smart string, force ...string) {
	res, err := s.API().EditUser(id, s.selection(smart, force))
	s.Require().NoError(err)
	s.Require().True(res.OK, "EditUser(%d): %s", id, res.Message())
}

// recreateUser re-provisions a user's clients over POST /admin/api/users/recreate and
// asserts {ok}.
func (s *UserSuite) recreateUser(id int64) {
	res, err := s.API().RecreateUser(id)
	s.Require().NoError(err)
	s.Require().True(res.OK, "RecreateUser(%d): %s", id, res.Message())
}

// subProxies fetches the user's node list over GET /sub/{token}/proxies (the absolute
// /sub URL the users API reports, plus the proxy-provider path) and parses the payload
// into a proxy name->uuid map — the same ground truth fleet.Sub would give, obtained
// purely over HTTP. Nodes are delivered as a proxy-provider, not inlined in /sub.
func (s *UserSuite) subProxies(u *api.User) map[string]string {
	subURL := u.Sub.SubURL()
	s.Require().NotEmpty(subURL, "user must have a subscription URL")

	resp, err := s.API().GetURL(subURL + "/proxies")
	s.Require().NoError(err)
	s.Require().Equal(200, resp.Status, "GET %s/proxies", subURL)

	px, err := api.SubProxies(resp.Body)
	s.Require().NoError(err)

	return px
}

// userName returns a unique, charset-valid nickname for a user scenario.
func (s *UserSuite) userName() string { return s.UniqueName("u") }
