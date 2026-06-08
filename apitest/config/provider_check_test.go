//go:build apitest

package config_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/postlog/subgen/apitest/api"
)

// Corner cases considered for POST /admin/api/config/mihomo/provider/check (a read-only
// reachability + content probe; it persists nothing). A local httptest file server
// started INSIDE the test serves sample mrs/yaml/text so the content checks need NO
// docker:
//   - ok.mrs / ok.yaml / ok.text  — reachable URL whose body matches the declared format
//                                    → {ok:true} "Доступен".
//   - format_mismatch             — body present but wrong shape for the format → {ok:false}.
//   - http_404                    — server returns 404 → {ok:false} "Сервер вернул HTTP 404".
//   - empty_body                  — 200 with an empty body → {ok:false} (no file).
//   - unreachable                 — connection refused on a closed port → {ok:false}.
//   - empty_url                   — "" → generic 400 (schema minLength:1, before the handler).
//   - malformed_json              — non-JSON body → generic 400.

// providerServer starts an in-test HTTP file server with a fixed set of sample
// rule-provider files (no docker). Paths:
//
//	/good.mrs   zstd-magic bytes (a valid .mrs container head)
//	/good.yaml  a payload list (the mihomo yaml rule-provider shape)
//	/good.text  plain rule lines
//	/empty      a 200 with an empty body
//	(any other path 404s)
func providerServer() *httptest.Server {
	zstd := []byte{0x28, 0xB5, 0x2F, 0xFD, 0x00} // zstd frame magic
	yamlBody := []byte("payload:\n  - '+.example.com'\n")
	textBody := []byte("# a comment\nexample.com\nexample.org\n")

	mux := http.NewServeMux()
	mux.HandleFunc("/good.mrs", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(zstd) })
	mux.HandleFunc("/good.yaml", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(yamlBody) })
	mux.HandleFunc("/good.text", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(textBody) })
	mux.HandleFunc("/empty", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	// Everything else 404s (net/http's default for an unmatched ServeMux path).

	return httptest.NewServer(mux)
}

// TestProviderCheck covers the probe against the in-test file server (no docker).
func (s *ConfigSuite) TestProviderCheck() {
	srv := providerServer()
	defer srv.Close()

	s.Run("ok.mrs", func() {
		res, err := s.api.CheckProvider(srv.URL+"/good.mrs", "mrs")
		s.Require().NoError(err)
		s.True(res.OK, "a valid mrs must be accepted: %s", res.Message())
		s.Contains(res.Msg, "Доступен")
	})

	s.Run("ok.yaml", func() {
		res, err := s.api.CheckProvider(srv.URL+"/good.yaml", "yaml")
		s.Require().NoError(err)
		s.True(res.OK, "a valid yaml must be accepted: %s", res.Message())
		s.Contains(res.Msg, "Доступен")
	})

	s.Run("ok.text", func() {
		res, err := s.api.CheckProvider(srv.URL+"/good.text", "text")
		s.Require().NoError(err)
		s.True(res.OK, "valid rule text must be accepted: %s", res.Message())
		s.Contains(res.Msg, "Доступен")
	})

	s.Run("format_mismatch", func() {
		// The yaml body is fetched but declared as mrs — content doesn't match.
		res, err := s.api.CheckProvider(srv.URL+"/good.yaml", "mrs")
		s.Require().NoError(err)
		s.False(res.OK, "a format mismatch must be rejected")
		s.Contains(res.Err, "не похоже на формат")
	})

	s.Run("http_404", func() {
		res, err := s.api.CheckProvider(srv.URL+"/missing.yaml", "yaml")
		s.Require().NoError(err)
		s.False(res.OK)
		s.Contains(res.Err, "HTTP 404")
	})

	s.Run("empty_body", func() {
		res, err := s.api.CheckProvider(srv.URL+"/empty", "text")
		s.Require().NoError(err)
		s.False(res.OK)
		s.Contains(res.Err, "пуст", "an empty body must report no file")
	})

	s.Run("unreachable", func() {
		// Connection refused on a closed loopback port.
		res, err := s.api.CheckProvider("http://127.0.0.1:1/x.yaml", "yaml")
		s.Require().NoError(err)
		s.False(res.OK, "an unreachable URL must report failure")
		s.NotEmpty(res.Err)
	})

	s.Run("empty_url", func() {
		// An empty URL trips the schema's minLength:1 before the handler runs → 400 generic.
		res, err := s.api.CheckProvider("", "yaml")
		s.Require().NoError(err)
		s.Equal(http.StatusBadRequest, res.Status)
		s.False(res.OK)
		s.Equal(api.MsgBadRequest, res.Err)
	})

	s.Run("malformed_json", func() {
		res, err := s.api.CheckProviderRaw([]byte("{not json"))
		s.Require().NoError(err)
		s.False(res.OK)
		s.Equal(api.MsgBadRequest, res.Err)
	})
}
