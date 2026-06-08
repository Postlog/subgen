//go:build apitest

package auth_test

import (
	"net/http"

	"github.com/postlog/subgen/apitest/api"
)

// Corner cases considered for the admin session gate (the ogen security handler on
// /admin/api/*) and the shell/static GETs:
//   - gate.read_api_401     — GET /admin/api/users without a session → 401 {errMessage}.
//   - gate.config_api_401   — GET /admin/api/config/mihomo without a session → 401.
//   - gate.mutation_401     — POST /admin/api/users/create without a session → 401 (the
//                             security check runs before the handler; no body needed).
//   - gate.authed_passes    — the same read API with a session → 200 JSON.
//   - shell.authed_index    — GET /admin (and a client-side view path) → 200 index.html.
//   - shell.unauthed_redirect — GET /admin without a session → 302 /admin/login (the SPA
//                             shell is a browser page, so it still redirects, not 401).
//   - static.public         — GET /admin/static/app.js → 200 (assets are NOT gated).

// TestSessionGate covers that every /admin/api/* route rejects an unauthenticated
// request with a 401, and lets an authenticated one through.
func (s *AuthSuite) TestSessionGate() {
	gated := []struct {
		name, method, path string
	}{
		{"users_read", http.MethodGet, "/admin/api/users"},
		{"nodes_read", http.MethodGet, "/admin/api/nodes"},
		{"config_read", http.MethodGet, "/admin/api/config/mihomo"},
		{"config_schema", http.MethodGet, "/admin/api/config/mihomo/schema"},
		{"users_create", http.MethodPost, "/admin/api/users/create"},
		{"nodes_save", http.MethodPost, "/admin/api/nodes/save"},
		{"config_save", http.MethodPost, "/admin/api/config/mihomo/save"},
		{"provider_check", http.MethodPost, "/admin/api/config/mihomo/provider/check"},
	}

	for _, g := range gated {
		s.Run("unauthed."+g.name, func() {
			var (
				resp api.Response
				err  error
			)

			if g.method == http.MethodGet {
				resp, err = s.anon.Get(g.path)
			} else {
				resp, err = s.anon.PostRaw(g.path, "application/json", []byte(`{}`))
			}

			s.Require().NoError(err)
			s.Equal(http.StatusUnauthorized, resp.Status, "%s %s must 401 when unauthenticated", g.method, g.path)

			decoded, derr := api.DecodeResult(resp.Body)
			s.Require().NoError(derr)
			s.Equal(api.MsgUnauthorized, decoded.Err)
		})
	}

	s.Run("authed_passes", func() {
		resp, err := s.authed.Get("/admin/api/users")
		s.Require().NoError(err)
		s.Equal(http.StatusOK, resp.Status, "an authenticated read API call must succeed")
		s.Contains(resp.Headers.Get("Content-Type"), "application/json")
	})
}

// TestShell covers the SPA shell: index.html for authed admin GETs, a redirect for
// anonymous ones.
func (s *AuthSuite) TestShell() {
	s.Run("authed_index", func() {
		for _, path := range []string{"/admin", "/admin/users", "/admin/config"} {
			resp, err := s.authed.Get(path)
			s.Require().NoError(err)
			s.Equal(http.StatusOK, resp.Status, "GET %s must serve the SPA shell", path)
			s.Contains(resp.Headers.Get("Content-Type"), "text/html")
		}
	})

	s.Run("unauthed_redirect", func() {
		resp, err := s.anon.Get("/admin")
		s.Require().NoError(err)
		s.Equal(http.StatusFound, resp.Status, "GET /admin must redirect when unauthenticated")
		s.Equal("/admin/login", resp.Headers.Get("Location"))
	})
}

// TestStaticPublic covers that the admin static assets are served without a session
// (they're mounted ahead of the gate).
func (s *AuthSuite) TestStaticPublic() {
	resp, err := s.anon.Get("/admin/static/app.js")
	s.Require().NoError(err)
	s.Equal(http.StatusOK, resp.Status, "static assets must be public (no session required)")
}
