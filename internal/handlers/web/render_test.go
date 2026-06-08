package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// staticModes covers both serving paths: the embedded copy (empty staticDir) and
// the on-disk override ("static" — resolved from the package dir, the test CWD).
var staticModes = map[string]string{"embedded": "", "disk": "static"}

// TestStaticHandler confirms the vendored assets + SPA are served from
// /admin/static/ (no CDN — RU1 DNS is unreliable), in both embedded and disk modes.
func TestStaticHandler(t *testing.T) {
	assets := []string{
		"/admin/static/bootstrap.min.css",
		"/admin/static/app.css",
		"/admin/static/app.js",
		"/admin/static/vue.global.prod.js",
		"/admin/static/js-yaml.min.js",
	}

	t.Parallel()

	for mode, dir := range staticModes {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()

			h := StaticHandler(dir)

			for _, p := range assets {
				rr := httptest.NewRecorder()
				h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, p, nil))

				assert.Equalf(t, http.StatusOK, rr.Code, "%s: status", p)
				assert.NotZerof(t, rr.Body.Len(), "%s: empty body", p)
			}
		})
	}
}

// TestReadPage confirms the SPA shell + login page are readable in both modes.
func TestReadPage(t *testing.T) {
	pages := []string{"index.html", "login.html"}

	t.Parallel()

	for mode, dir := range staticModes {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()

			for _, name := range pages {
				b, err := ReadPage(dir, name)

				require.NoErrorf(t, err, "%s", name)
				assert.NotZerof(t, len(b), "%s: empty body", name)
			}
		})
	}
}
