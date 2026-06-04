//go:build apitest

package sub_test

import "net/http"

// Corner cases considered for GET /sub/{token} that need NO panel (the valid-token case
// is in sub_valid_test.go, gated):
//   - malformed_token — a token that is not a valid HMAC for any subId → 404.
//   - unknown_subid   — a well-formed-looking but unmatched token → 404 (no user owns it).
//   - empty_token_path — /sub/ with no token → not routed (404).
//
// The handler resolves the token against service-owned users only and returns
// http.NotFound for anything it can't match, so all of these are a clean 404 with no
// body leak.

// TestSubUnknownToken covers the 404 paths: an unmatched token never renders a profile.
func (s *SubSuite) TestSubUnknownToken() {
	s.Run("malformed_token", func() {
		resp, err := s.api.Get("/sub/not-a-real-token")
		s.Require().NoError(err)
		s.Equal(http.StatusNotFound, resp.Status, "a token matching no subId must 404")
	})

	s.Run("unknown_subid", func() {
		// A hex string that could be a token but matches no user (the store is empty).
		resp, err := s.api.Get("/sub/0123456789abcdef0123456789abcdef")
		s.Require().NoError(err)
		s.Equal(http.StatusNotFound, resp.Status)
	})

	s.Run("empty_token_path", func() {
		// "/sub/" with an empty token is not a matched route → 404.
		resp, err := s.api.Get("/sub/")
		s.Require().NoError(err)
		s.Equal(http.StatusNotFound, resp.Status)
	})
}
