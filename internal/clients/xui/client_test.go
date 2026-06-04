package xui

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/postlog/subgen/internal/entity"
)

// newTestClient spins up an httptest server running h and returns a Client plus a
// PanelTarget pointed at it. Server cleanup is registered on t.
func newTestClient(t *testing.T, h http.HandlerFunc) (*Client, entity.PanelTarget) {
	t.Helper()

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	return New(), entity.PanelTarget{BaseURL: srv.URL, Token: "tok"}
}
