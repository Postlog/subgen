//go:build apitest

package sub_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"time"

	"github.com/postlog/subgen/apitest/api"
)

// Corner cases considered for GET /rules/{file}:
//   - unknown_file — a file no mirrored provider produces → 404 (the empty-store default).
//   - present      — a mirrored provider's file IS served (200 + its bytes). The served
//                    set is fixed at startup from the store, so this uses two boots on one
//                    DB: boot #1 saves a mirror provider (pointing at an in-test upstream)
//                    via the API; boot #2 on the same DB picks it up and mirrors it.
//   - empty_file_path — /rules/ with no file → 404 (unrouted).
//
// All of this needs NO docker panel — only the admin API + a local upstream server.

// TestRulesUnknownFile covers the 404 path on the bare (empty-store) server.
func (s *SubSuite) TestRulesUnknownFile() {
	s.Run("unknown_file", func() {
		resp, err := s.api.Get("/rules/nope.yaml")
		s.Require().NoError(err)
		s.Equal(http.StatusNotFound, resp.Status, "an unmirrored file must 404")
	})

	s.Run("empty_file_path", func() {
		// /rules/ has an empty {file} path param; the spec marks it minLength:1, so the
		// ogen router answers 400 (malformed request), not 404.
		resp, err := s.api.Get("/rules/")
		s.Require().NoError(err)
		s.Equal(http.StatusBadRequest, resp.Status)
	})
}

// TestRulesMirroredPresent covers a PRESENT mirrored file end-to-end without docker: an
// in-test upstream serves a yaml rule file; a mirror provider for it is saved via the
// API on a shared DB; a second server boot on that DB mirrors it and serves it at
// /rules/<name>.yaml.
func (s *SubSuite) TestRulesMirroredPresent() {
	const providerName = "testmirror"

	body := []byte("payload:\n  - '+.mirrored.example'\n")

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer upstream.Close()

	// A DB path shared across the two boots.
	dbPath := filepath.Join(s.T().TempDir(), "mirror.db")

	// Boot #1: save a mirror provider pointing at the upstream, then stop.
	seed := api.StartServerWith(s.T(), api.Options{DBPath: dbPath})
	seedClient := api.New(seed.BaseURL())
	res, err := seedClient.Login(api.AdminUser, api.AdminPass)
	s.Require().NoError(err)
	s.Require().True(res.OK)

	save, err := seedClient.SaveConfig(api.Config{
		Providers: []api.ConfigProvider{{
			Name: providerName, Behavior: "domain", Format: "yaml",
			URL: upstream.URL + "/rule.yaml", Interval: 3600,
			Mirror: true, MirrorInterval: 3600,
		}},
	})
	s.Require().NoError(err)
	s.Require().True(save.OK, "save mirror provider: %s", save.Message())
	seed.Stop()

	// Boot #2 on the same DB: the mirror set now includes the provider, fetched on start.
	srv := api.StartServerWith(s.T(), api.Options{DBPath: dbPath})
	c := api.New(srv.BaseURL())

	// The initial mirror fetch is async; poll briefly for the file to appear.
	var resp api.Response

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err = c.Get("/rules/" + providerName + ".yaml")
		s.Require().NoError(err)

		if resp.Status == http.StatusOK {
			break
		}

		time.Sleep(150 * time.Millisecond)
	}

	s.Require().Equal(http.StatusOK, resp.Status, "the mirrored file must be served after boot")
	s.Equal(body, resp.Body, "the served bytes must match the upstream file")
	// ogen pins the 200 media type from the spec (application/octet-stream) — the mirror's
	// per-file Content-Type is no longer echoed — but the nosniff guard is still set.
	s.Equal("nosniff", resp.Headers.Get("X-Content-Type-Options"))
}
