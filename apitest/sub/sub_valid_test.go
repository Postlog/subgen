//go:build apitest

package sub_test

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/postlog/subgen/apitest/api"
)

// Corner cases considered for a VALID GET /sub/{token} (gated — provisions a user):
//   - body        — the response is the rendered mihomo YAML, and its proxies carry the
//                   real per-inbound client uuids.
//   - headers     — Content-Type text/yaml; a base64 Profile-Title; a Content-Disposition
//                   filename; a Profile-Update-Interval; and a Subscription-Userinfo line.
//
// The profile knobs are operator-set (no code defaults), so the test first saves a base
// config with an explicit profile and asserts the response headers echo exactly that.

// Profile knobs saved on the base config and expected back in the subscription headers.
const (
	subProfileTitle    = "Fleet"
	subProfileFilename = "fleet.yaml"
	subProfileInterval = "4"
)

// TestSubValid covers the happy subscription fetch for a provisioned user.
func (s *SubPanelSuite) TestSubValid() {
	// The profile is operator-set: configure the base config so the subscription has
	// concrete header values (there are no server-side defaults).
	cfgRes, err := s.API().SaveConfig(api.Config{
		ProfileTitle: subProfileTitle, Filename: subProfileFilename, ProfileUpdateInterval: 4,
	})
	s.Require().NoError(err)
	s.Require().True(cfgRes.OK, "save base config: %s", cfgRes.Message())

	name := s.UniqueName("su")
	res, err := s.API().CreateUser(name, []int64{s.InboundID("N1", "smart")})
	s.Require().NoError(err)
	s.Require().True(res.OK, "provision user: %s", res.Message())

	u, err := s.API().MustFindUser(name)
	s.Require().NoError(err)
	s.T().Cleanup(func() { _, _ = s.API().DeleteUser(u.ID) })
	subURL := u.Sub.SubURL()
	s.Require().NotEmpty(subURL, "user must have a /sub URL")

	resp, err := s.API().GetURL(subURL)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, resp.Status, "GET %s", subURL)

	s.Run("headers", func() {
		s.Contains(resp.Headers.Get("Content-Type"), "text/yaml")

		// Profile-Title is base64 of the configured title.
		pt := resp.Headers.Get("Profile-Title")
		s.True(strings.HasPrefix(pt, "base64:"), "Profile-Title must be base64-prefixed: %q", pt)
		dec, derr := base64.StdEncoding.DecodeString(strings.TrimPrefix(pt, "base64:"))
		s.Require().NoError(derr)
		s.Equal(subProfileTitle, string(dec))

		// Content-Disposition carries the configured filename.
		s.Contains(resp.Headers.Get("Content-Disposition"), `filename="`+subProfileFilename+`"`)

		// The refresh hint and the traffic line are present.
		s.Equal(subProfileInterval, resp.Headers.Get("Profile-Update-Interval"))
		s.Contains(resp.Headers.Get("Subscription-Userinfo"), "upload=")
		s.Contains(resp.Headers.Get("Subscription-Userinfo"), "download=")
	})

	s.Run("body", func() {
		px, perr := api.SubProxies(resp.Body)
		s.Require().NoError(perr, "subscription must be valid YAML")
		s.Contains(px, "N1-smart", "the provisioned inbound must appear as a proxy")
		s.Equal(s.ClientUUID(s.Pan1(), api.N1Smart, name), px["N1-smart"], "proxy uuid must match the real client")
	})
}
