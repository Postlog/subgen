//go:build apitest

package api

import (
	"bytes"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Test topology, created by Base.SetupSuite on the docker panels (docker-compose.yml):
//
//	N1 (panel1): smart inbound :4433, force inbound :8443
//	N2 (panel2): smart inbound :9443, force inbound :9444
const (
	N1Smart = 4433
	N1Force = 8443
	N2Smart = 9443
	N2Force = 9444
)

// Test admin credentials handed to the spawned server via env, and used by the SDK to
// log in. Arbitrary — the server is ephemeral and loopback-only.
const (
	AdminUser = "apitest-admin"
	AdminPass = "apitest-secret-pw"
	HMACKey   = "apitest-hmac-secret-0123456789abcdef"
)

// Server is a spawned subgen process plus the base URL to reach it and its captured
// log (surfaced on a failed boot).
type Server struct {
	cmd     *exec.Cmd
	baseURL string
	logBuf  *bytes.Buffer
}

// BaseURL returns the loopback URL the spawned subgen listens on.
func (sp *Server) BaseURL() string { return sp.baseURL }

// Log returns everything the spawned process wrote to stdout/stderr so far.
func (sp *Server) Log() string {
	if sp == nil || sp.logBuf == nil {
		return ""
	}

	return sp.logBuf.String()
}

// Options tunes a spawned server. The zero value is the default StartServer config (a
// fresh temp DB). DBPath lets a test reuse a store across two boots — needed to test
// the rule-provider MIRROR, whose served-file set is fixed at startup from the store:
// boot once to save a mirror provider via the API, stop, then boot again on the SAME
// DBPath so the mirror picks it up.
type Options struct {
	DBPath string // SQLite path; empty => a fresh file in t.TempDir()
}

// StartServer builds the subgen binary and runs it as a subprocess on a free loopback
// port with a temp SQLite db and the test admin creds. See StartServerWith for the
// details; this is the zero-Options form.
func StartServer(t *testing.T) *Server {
	t.Helper()
	return StartServerWith(t, Options{})
}

// StartServerWith builds the subgen binary into a temp dir and starts it as a
// subprocess on a free loopback port with the given Options, the test admin creds, and
// the test HMAC secret. It polls /healthz until ready, registers cleanup that stops the
// process, and returns the handle. A failed build/boot fails the test fast.
//
// The admin panel is mounted (AdminUser+AdminPass are set), TLS is off (plain HTTP),
// and SUBGEN_CACHE_TTL=0 so panel changes are seen immediately. SUBGEN_PUBLIC_BASE is
// the loopback base, so the users API emits absolute /sub URLs the SDK can GET.
func StartServerWith(t *testing.T, opts Options) *Server {
	t.Helper()

	bin := filepath.Join(t.TempDir(), "subgen")

	build := exec.Command("go", "build", "-o", bin, "./cmd/service")

	build.Dir = moduleRoot(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build subgen: %v\n%s", err, out)
	}

	port := freePort(t)
	addr := net.JoinHostPort("127.0.0.1", port)
	base := "http://" + addr

	dbPath := opts.DBPath
	if dbPath == "" {
		dbPath = filepath.Join(t.TempDir(), "subgen.db")
	}

	logBuf := &bytes.Buffer{}

	cmd := exec.Command(bin, "-env", os.DevNull) //nolint:gosec // bin is our own freshly built binary
	cmd.Dir = t.TempDir()

	cmd.Env = append(os.Environ(),
		"SUBGEN_LISTEN="+addr,
		"SUBGEN_DB_PATH="+dbPath,
		"SUBGEN_SECRET="+HMACKey,
		"SUBGEN_ADMIN_USER="+AdminUser,
		"SUBGEN_ADMIN_PASSWORD="+AdminPass,
		"SUBGEN_PUBLIC_BASE="+base,
		"SUBGEN_CACHE_TTL=0",
	)
	cmd.Stdout = logBuf
	cmd.Stderr = logBuf

	if err := cmd.Start(); err != nil {
		t.Fatalf("start subgen: %v", err)
	}

	sp := &Server{cmd: cmd, baseURL: base, logBuf: logBuf}
	t.Cleanup(sp.Stop)

	// Stop the process if readiness polling fails, so a failed boot doesn't leak it.
	if !waitHealthy(New(base), 20*time.Second) {
		sp.Stop()
		t.Fatalf("subgen did not become healthy at %s\n--- server log ---\n%s", base, logBuf.String())
	}

	return sp
}

// Stop signals the subgen process to exit and waits for it (best-effort). It is
// registered as test cleanup, and safe to call twice.
func (sp *Server) Stop() {
	if sp == nil || sp.cmd == nil || sp.cmd.Process == nil {
		return
	}

	_ = sp.cmd.Process.Signal(os.Interrupt)

	done := make(chan struct{})

	go func() { _, _ = sp.cmd.Process.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = sp.cmd.Process.Kill()
	}
}

// waitHealthy polls GET /healthz until it returns ok or the timeout elapses.
func waitHealthy(c *Client, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if c.Healthz() {
			return true
		}

		time.Sleep(100 * time.Millisecond)
	}

	return false
}

// moduleRoot walks up from the test's working dir to the directory holding go.mod
// (the subgen module root), where `go build ./cmd/service` runs. Each area test
// package runs from apitest/<area>/, so the root is two-plus levels up — walking to
// go.mod is robust to that nesting.
func moduleRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("module root (go.mod) not found above %s", dir)
		}

		dir = parent
	}
}

// freePort asks the OS for an unused TCP port on loopback and returns it as a string.
func freePort(t *testing.T) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve free port: %v", err)
	}

	defer func() { _ = l.Close() }()

	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}

	return port
}
