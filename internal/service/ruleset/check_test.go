package ruleset

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/postlog/subgen/internal/entity"
)

func TestCheckker_Check(t *testing.T) {
	zstd := []byte{0x28, 0xB5, 0x2F, 0xFD, 0x00} // zstd magic = a valid .mrs container head
	yamlBody := []byte("payload:\n  - '+.example.com'\n")
	textBody := []byte("# comment\nexample.com\n")

	tt := []struct {
		name    string
		handler http.HandlerFunc
		format  string
		url     string // set => skip the test server (used for the unreachable case)
		want    entity.RulesetCheckOutcome
	}{
		{name: "ok.mrs", format: "mrs", handler: serve(200, zstd), want: entity.RulesetCheckOK},
		{name: "ok.yaml", format: "yaml", handler: serve(200, yamlBody), want: entity.RulesetCheckOK},
		{name: "ok.text", format: "text", handler: serve(200, textBody), want: entity.RulesetCheckOK},
		{name: "http_error", format: "text", handler: serve(404, []byte("nope")), want: entity.RulesetCheckHTTPError},
		{name: "empty", format: "text", handler: serve(200, nil), want: entity.RulesetCheckEmpty},
		{name: "format_mismatch.mrs", format: "mrs", handler: serve(200, textBody), want: entity.RulesetCheckFormatMismatch},
		{name: "format_mismatch.yaml_html", format: "yaml", handler: serve(200, []byte("<html>404</html>")), want: entity.RulesetCheckFormatMismatch},
		{name: "format_mismatch.yaml_payload_not_list", format: "yaml", handler: serve(200, []byte("payload: just-a-string\n")), want: entity.RulesetCheckFormatMismatch},
		{name: "format_mismatch.text_html", format: "text", handler: serve(200, []byte("<!doctype html><h1>x</h1>")), want: entity.RulesetCheckFormatMismatch},
		{name: "format_mismatch.text_invalid_utf8", format: "text", handler: serve(200, []byte{0xff, 0xfe, 0xfd}), want: entity.RulesetCheckFormatMismatch},
		{name: "unreachable", format: "text", url: "http://127.0.0.1:1/x", want: entity.RulesetCheckUnreachable},
		{name: "unreachable.blank_url", format: "text", url: "   ", want: entity.RulesetCheckUnreachable},           // blank is just un-probeable
		{name: "unreachable.malformed_url", format: "text", url: "://nohost", want: entity.RulesetCheckUnreachable}, // same category as blank
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			url := tc.url
			if url == "" {
				srv := httptest.NewServer(tc.handler)
				defer srv.Close()

				url = srv.URL
			}

			res := NewChecker().Check(context.Background(), url, tc.format)
			assert.Equal(t, tc.want, res.Outcome)
		})
	}
}

func serve(status int, body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}
}
