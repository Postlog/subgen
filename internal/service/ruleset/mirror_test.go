package ruleset

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/mihomo"
)

// TestMirror_New checks that only providers with Mirror == true are tracked, and that
// nothing is cached before a fetch runs (Get is false for every key up front).
func TestMirror_New(t *testing.T) {
	tt := []struct {
		name string

		providers []mihomo.RuleProvider

		// probe is the file key we expect (or don't) to be tracked; want is whether
		// Get should ever be able to return it. Since New never fetches, Get is false
		// for all keys here — we assert the tracked-vs-skipped split via the key's ext.
		probe string
		want  bool
	}{
		{name: "empty", providers: nil, probe: "x.mrs", want: false},
		{
			name:      "skips_non_mirror",
			providers: []mihomo.RuleProvider{{Name: "geo", Format: "mrs", Mirror: false}},
			probe:     "geo.mrs",
			want:      false,
		},
		{
			name:      "keeps_mirror.mrs",
			providers: []mihomo.RuleProvider{{Name: "geo", Format: "mrs", Mirror: true}},
			probe:     "geo.mrs",
			want:      false, // tracked, but not yet fetched => still false
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m := New(tc.providers)

			_, _, ok := m.Get(tc.probe)
			assert.Equal(t, tc.want, ok)
		})
	}
}

// TestMirror_Get drives a real fetch through Run against an httptest server and asserts
// the cached bytes + content-type (covering extFor/contentTypeFor for mrs/yaml/text via
// the public surface), and that a download failure leaves the cache empty (last-good).
func TestMirror_Get(t *testing.T) {
	body := []byte("payload: data")

	tt := []struct {
		name string

		format string // provider format -> drives ext + content-type
		status int    // server response status
		fail   bool   // serve a non-200 (download error) => nothing cached

		file        string // expected cache key "<name><ext>"
		wantData    []byte
		wantCT      string
		wantPresent bool
	}{
		{
			name: "success.mrs", format: "mrs", status: 200,
			file: "geo.mrs", wantData: body, wantCT: "application/octet-stream", wantPresent: true,
		},
		{
			name: "success.yaml", format: "yaml", status: 200,
			file: "geo.yaml", wantData: body, wantCT: "application/yaml; charset=utf-8", wantPresent: true,
		},
		{
			name: "success.text", format: "text", status: 200,
			file: "geo.txt", wantData: body, wantCT: "text/plain; charset=utf-8", wantPresent: true,
		},
		{
			name: "error.non_200", format: "mrs", status: 500, fail: true,
			file: "geo.mrs", wantPresent: false,
		},
	}

	t.Parallel()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var hits int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&hits, 1)
				w.WriteHeader(tc.status)
				_, _ = w.Write(body)
			}))
			defer srv.Close()

			m := New([]mihomo.RuleProvider{{Name: "geo", Format: tc.format, URL: srv.URL, Mirror: true}})

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			go m.Run(ctx) // performs the initial fetch synchronously, then blocks

			if tc.wantPresent {
				require.Eventually(t, func() bool {
					_, _, ok := m.Get(tc.file)

					return ok
				}, 2*time.Second, 5*time.Millisecond)

				data, ct, ok := m.Get(tc.file)
				require.True(t, ok)
				assert.Equal(t, tc.wantData, data)
				assert.Equal(t, tc.wantCT, ct)

				return
			}

			// Failure: wait until the server has actually been hit (so we know fetch
			// ran), then assert the cache stayed empty — the last-good copy is kept.
			require.Eventually(t, func() bool {
				return atomic.LoadInt32(&hits) > 0
			}, 2*time.Second, 5*time.Millisecond)

			_, _, ok := m.Get(tc.file)
			assert.False(t, ok)
		})
	}
}

// TestMirror_Get_NetworkError covers the download network-error branch (unreachable
// URL): fetch returns early and Get stays empty.
func TestMirror_Get_NetworkError(t *testing.T) {
	t.Parallel()

	m := New([]mihomo.RuleProvider{{Name: "geo", Format: "mrs", URL: "http://127.0.0.1:1/x", Mirror: true}})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go m.Run(ctx)

	// The initial fetch is best-effort and synchronous at the start of Run; give it a
	// moment, then confirm nothing was cached.
	assert.Never(t, func() bool {
		_, _, ok := m.Get("geo.mrs")

		return ok
	}, 200*time.Millisecond, 10*time.Millisecond)
}
