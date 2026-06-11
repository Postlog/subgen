//go:build apitest

package sub_test

import "net/http"

// Corner cases considered for GET /sub/{kind}/{token} that need NO panel (the valid-token
// case is in sub_valid_test.go, gated):
//   - malformed_token — a token that is not a valid HMAC for any subId → 404.
//   - unknown_subid   — a well-formed-looking but unmatched token → 404 (no user owns it).
//   - unknown_kind    — a valid token shape but an unregistered engine → 404.
//   - legacy_path     — the old /sub/{token} (no engine) is gone → 404 (one segment, unrouted).
//   - empty_token_path — /sub/mihomo/ with an empty {token} segment → 400 (the ogen router
//                        rejects an empty path segment as a malformed request, before any handler).
//
// The handler resolves the engine against the renderer registry and the token against
// service-owned users only, returning http.NotFound for anything it can't match, so all
// of these are a clean 404 with no body leak.

// TestSubUnknownToken covers the 404 paths: an unmatched engine/token never renders a profile.
func (s *SubSuite) TestSubUnknownToken() {
	s.Run("malformed_token", func() {
		resp, err := s.api.Get("/sub/mihomo/not-a-real-token")
		s.Require().NoError(err)
		s.Equal(http.StatusNotFound, resp.Status, "a token matching no subId must 404")
	})

	s.Run("unknown_subid", func() {
		// A hex string that could be a token but matches no user (the store is empty).
		resp, err := s.api.Get("/sub/mihomo/0123456789abcdef0123456789abcdef")
		s.Require().NoError(err)
		s.Equal(http.StatusNotFound, resp.Status)
	})

	s.Run("unknown_kind", func() {
		// A registered-looking token but an engine that has no renderer → 404.
		resp, err := s.api.Get("/sub/noname/0123456789abcdef0123456789abcdef")
		s.Require().NoError(err)
		s.Equal(http.StatusNotFound, resp.Status)
	})

	s.Run("legacy_path", func() {
		// The old engine-less route is removed.
		resp, err := s.api.Get("/sub/0123456789abcdef0123456789abcdef")
		s.Require().NoError(err)
		s.Equal(http.StatusNotFound, resp.Status)
	})

	s.Run("empty_token_path", func() {
		// "/sub/mihomo/" has an empty {token} path segment; the ogen router rejects it as a
		// malformed request → 400 (not 404), independent of any field validation.
		resp, err := s.api.Get("/sub/mihomo/")
		s.Require().NoError(err)
		s.Equal(http.StatusBadRequest, resp.Status)
	})
}
