// Package xui is a thin, stateless adapter to the 3x-ui panel HTTP API (>= 3.2),
// authenticating with a Bearer API token (no session/CSRF needed). It holds only a
// shared http.Client; the panel to talk to (base URL + token) is passed per call
// as an entity.PanelTarget, so one Client serves the whole fleet — no per-node
// state, no business logic. Each public method lives in its own file with a test.
package xui

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/postlog/subgen/internal/entity"
)

// Client talks to 3x-ui panels using a Bearer API token. Token-authenticated
// requests bypass CSRF and need no login round-trip.
type Client struct {
	hc *http.Client
}

// New builds a client. TLS verification is skipped because panels commonly serve
// their own cert and subgen talks to them over loopback/LAN.
func New() *Client {
	return &Client{
		hc: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // panels serve their own cert over loopback/LAN
				DialContext:     (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
			},
		},
	}
}

// reqURL joins the target's base URL + base path with the API path.
func reqURL(t entity.PanelTarget, path string) string {
	base := strings.TrimRight(t.BaseURL, "/") + "/" + strings.Trim(t.BasePath, "/")
	base = strings.TrimRight(base, "/")

	return base + path
}

// get performs an authenticated GET against the target and returns the body.
func (c *Client) get(ctx context.Context, t entity.PanelTarget, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL(t, path), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+t.Token)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", path, err)
	}

	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<24))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d", path, resp.StatusCode)
	}

	return body, nil
}

// postJSON performs an authenticated POST and checks the {success,msg} envelope.
func (c *Client) postJSON(ctx context.Context, t entity.PanelTarget, path string, body any) error {
	var rd io.Reader

	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}

		rd = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL(t, path), rd)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+t.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", path, err)
	}

	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("POST %s: status %d", path, resp.StatusCode)
	}

	var r struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
	}

	if err := json.Unmarshal(raw, &r); err != nil {
		return fmt.Errorf("POST %s: parse: %w", path, err)
	}

	if !r.Success {
		return fmt.Errorf("POST %s: %s", path, strings.TrimSpace(r.Msg))
	}

	return nil
}
