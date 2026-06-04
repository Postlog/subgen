//go:build apitest

// Package api is the shared support for subgen's black-box API tests. It is a normal
// (build-tagged, non-_test) package the per-area test packages import, so every area
// codes against one stable SDK + suite contract.
//
// It provides three things:
//
//   - Client — a typed HTTP SDK for a running subgen server: one method per endpoint
//     the suites drive, decoding the {ok,msg|err} envelope (mutations) or the typed
//     read shapes (User/Node/Config/…). See client.go + the per-area *.go files here.
//   - Server boot — StartServer(t) builds the real subgen binary and runs it as a
//     subprocess on a free loopback port with a temp SQLite store and the test admin
//     creds; it polls /healthz and registers cleanup. See server.go.
//   - Base — a testify suite an area embeds to get a booted server, an authenticated
//     SDK, the two registered docker panels, and the 3x-ui ground-truth probing
//     (clientUUID/RequireClient/…). See base.go + probe.go.
//
// Nothing here reaches into subgen's services: the only way in is the same HTTP API
// the operator/SPA use. The one direct dependency on internal/* is the xui client,
// used solely to read panel ground truth (and seed a deliberate collision).
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"time"
)

// AdminCookie is the name of the session cookie subgen issues on login
// (internal/handlers/web/auth.go). It is flagged Secure, so Go's cookie jar will not
// replay it over plain http://; the Client therefore captures it from the login
// response and attaches it to every subsequent request by hand. This is legitimate
// black-box behaviour — over real HTTPS a browser/jar would send it automatically;
// over loopback HTTP we replay it explicitly, without touching production code.
const AdminCookie = "subgen_admin"

// Result is the {ok, msg|err} envelope every mutation handler returns (see
// web.WriteJSON). Exactly one of Msg/Err is populated, keyed by OK.
type Result struct {
	OK  bool   `json:"ok"`
	Msg string `json:"msg"`
	Err string `json:"err"`
}

// Message returns the human-facing text of the result (Msg on success, Err on
// failure) — handy for asserting which message the API produced.
func (r Result) Message() string {
	if r.OK {
		return r.Msg
	}

	return r.Err
}

// Client is the black-box HTTP SDK for a running subgen server. It holds the base
// URL, a cookie jar and the captured admin session, and exposes one typed method per
// endpoint the suites drive. Construct it with New; sign in with Login before calling
// the gated /admin endpoints.
type Client struct {
	base    string
	hc      *http.Client
	session *http.Cookie // captured subgen_admin cookie (see AdminCookie)
}

// New builds an SDK client for the subgen server at base (e.g.
// "http://127.0.0.1:34567"). It uses a cookie jar plus an explicit session-cookie
// capture so the admin session survives over plain HTTP. Redirects are NOT followed,
// so an unauthenticated /admin call surfaces as a 302 rather than chasing the login
// page — area tests assert on that redirect.
func New(base string) *Client {
	jar, _ := cookiejar.New(nil)

	return &Client{
		base: base,
		hc: &http.Client{
			Timeout:       30 * time.Second,
			Jar:           jar,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

// BaseURL returns the server base URL this client targets.
func (c *Client) BaseURL() string { return c.base }

// Authed reports whether the client currently holds a captured admin session cookie
// (i.e. a successful Login happened).
func (c *Client) Authed() bool { return c.session != nil }

// ---- request core ----------------------------------------------------------

// Response is the raw outcome of an HTTP call: status, body and headers, for callers
// that need more than a decoded struct (the /sub YAML + headers, redirect assertions,
// the login page HTML, …).
type Response struct {
	Status  int
	Body    []byte
	Headers http.Header
}

// do performs method+path with an optional JSON request body. If out is non-nil the
// response body is JSON-decoded into it. It returns the raw status+body+headers so
// callers can assert on the status code too. The captured admin session cookie (if
// any) is attached to every request.
func (c *Client) do(method, path string, reqBody, out any) (Response, error) {
	var rd io.Reader

	if reqBody != nil {
		b, err := json.Marshal(reqBody)
		if err != nil {
			return Response{}, fmt.Errorf("marshal request: %w", err)
		}

		rd = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, c.base+path, rd)
	if err != nil {
		return Response{}, fmt.Errorf("new request: %w", err)
	}

	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.session != nil {
		req.AddCookie(c.session)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("%s %s: %w", method, path, err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<24))
	if err != nil {
		return Response{}, fmt.Errorf("%s %s: read body: %w", method, path, err)
	}

	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return Response{}, fmt.Errorf("%s %s: decode %d body %q: %w", method, path, resp.StatusCode, truncate(body), err)
		}
	}

	return Response{Status: resp.StatusCode, Body: body, Headers: resp.Header}, nil
}

// Get performs a raw GET and returns the status/body/headers without decoding. Used
// by area tests that assert on the raw response (the SPA shell HTML, the login page,
// the /admin redirect, the rules file bytes).
func (c *Client) Get(path string) (Response, error) {
	return c.do(http.MethodGet, path, nil, nil)
}

// PostRaw sends an arbitrary (already-serialised) body with the given content-type and
// returns the raw response — for the malformed-JSON cases that can't go through the
// typed helpers. A nil body sends no payload.
func (c *Client) PostRaw(path, contentType string, body []byte) (Response, error) {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}

	req, err := http.NewRequest(http.MethodPost, c.base+path, rd)
	if err != nil {
		return Response{}, fmt.Errorf("new request: %w", err)
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	if c.session != nil {
		req.AddCookie(c.session)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("POST %s: %w", path, err)
	}

	defer resp.Body.Close()

	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<24))
	if err != nil {
		return Response{}, fmt.Errorf("POST %s: read body: %w", path, err)
	}

	return Response{Status: resp.StatusCode, Body: b, Headers: resp.Header}, nil
}

// post sends a JSON body and decodes the {ok,…} envelope into a Result.
func (c *Client) post(path string, reqBody any) (Result, error) {
	var res Result

	if _, err := c.do(http.MethodPost, path, reqBody, &res); err != nil {
		return Result{}, err
	}

	return res, nil
}

// getJSON GETs path and decodes the JSON response into out.
func (c *Client) getJSON(path string, out any) error {
	_, err := c.do(http.MethodGet, path, nil, out)
	return err
}

// MsgBadRequest is the user-facing message subgen returns for a malformed request body
// (mirrors web.MsgBadRequest at the presentation layer). Black-box tests assert the
// exact text the API produces; re-stating it here keeps each area package from
// importing the internal handler packages.
const MsgBadRequest = "некорректный JSON в запросе"

// DecodeResult unmarshals a raw {ok,msg|err} body into a Result (for the *Raw helpers
// and any test that posts a hand-built body and still wants the typed envelope back).
func DecodeResult(body []byte) (Result, error) {
	return decodeResult(body)
}

// decodeResult unmarshals a raw {ok,msg|err} body into a Result.
func decodeResult(body []byte) (Result, error) {
	var res Result
	if err := json.Unmarshal(body, &res); err != nil {
		return Result{}, fmt.Errorf("decode result %q: %w", truncate(body), err)
	}

	return res, nil
}

func truncate(b []byte) string {
	const max = 256
	if len(b) > max {
		return string(b[:max]) + "…"
	}

	return string(b)
}

// ---- auth -------------------------------------------------------------------

// Login posts the admin credentials to /admin/login and, on success, captures the
// session cookie so subsequent /admin calls are authenticated. It returns the
// handler's {ok,…} Result; the caller asserts on it. A failed login leaves the client
// unauthenticated.
func (c *Client) Login(user, password string) (Result, error) {
	return c.loginJSON(map[string]string{"user": user, "password": password})
}

// LoginRaw posts an arbitrary JSON login body (for malformed/missing-field cases). It
// captures the session cookie if one comes back and returns the {ok,…} Result plus the
// HTTP status.
func (c *Client) LoginRaw(body []byte) (Result, int, error) {
	res, status, err := c.loginBytes(body, "application/json")
	return res, status, err
}

func (c *Client) loginJSON(req any) (Result, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return Result{}, fmt.Errorf("marshal login: %w", err)
	}

	res, _, err := c.loginBytes(b, "application/json")

	return res, err
}

// loginBytes posts the given body to /admin/login, captures the admin cookie, and
// decodes the envelope. It is the single place login goes out so cookie capture is
// consistent across Login/LoginRaw.
func (c *Client) loginBytes(body []byte, contentType string) (Result, int, error) {
	req, err := http.NewRequest(http.MethodPost, c.base+"/admin/login", bytes.NewReader(body))
	if err != nil {
		return Result{}, 0, fmt.Errorf("new login request: %w", err)
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return Result{}, 0, fmt.Errorf("POST /admin/login: %w", err)
	}

	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	for _, ck := range resp.Cookies() {
		if ck.Name == AdminCookie {
			c.session = &http.Cookie{Name: ck.Name, Value: ck.Value}
		}
	}

	var res Result
	if err := json.Unmarshal(raw, &res); err != nil {
		return Result{}, resp.StatusCode, fmt.Errorf("decode login body %q: %w", truncate(raw), err)
	}

	return res, resp.StatusCode, nil
}

// ---- health -----------------------------------------------------------------

// Healthz GETs /healthz and reports whether the server answered 200 "ok". Used to
// poll for readiness during startup.
func (c *Client) Healthz() bool {
	resp, err := c.do(http.MethodGet, "/healthz", nil, nil)
	return err == nil && resp.Status == http.StatusOK && bytes.Contains(resp.Body, []byte("ok"))
}

// HealthzRaw GETs /healthz and returns the raw response (for asserting status + body
// in the health area test).
func (c *Client) HealthzRaw() (Response, error) {
	return c.do(http.MethodGet, "/healthz", nil, nil)
}

// GetURL GETs an absolute URL (as reported by the users API for /sub) and returns the
// raw status + body + headers. Used for the subscription fetch and any other absolute
// URL a read endpoint hands back.
func (c *Client) GetURL(rawURL string) (Response, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return Response{}, fmt.Errorf("new request: %w", err)
	}

	if c.session != nil {
		req.AddCookie(c.session)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("GET %s: %w", rawURL, err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<24))
	if err != nil {
		return Response{}, fmt.Errorf("GET %s: read body: %w", rawURL, err)
	}

	return Response{Status: resp.StatusCode, Body: body, Headers: resp.Header}, nil
}
