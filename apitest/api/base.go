//go:build apitest

package api

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/postlog/subgen/internal/clients/xui"
	"github.com/postlog/subgen/internal/entity"
)

// Ctx is the shared background context for the direct-panel (xui) probing calls.
var Ctx = context.Background()

// SkipUnlessConfigured skips a panel-dependent suite/test unless the docker panels are
// wired up via env (the Makefile sets these after `docker compose up`). A suite that
// provisions onto a panel calls this in its runner; a test that needs no panel (login
// errors, auth redirects, provider-check content) skips the gate so it runs in plain
// CI.
func SkipUnlessConfigured(t *testing.T) {
	if os.Getenv("SUBGEN_APITEST_PANEL1_URL") == "" {
		t.Skip("apitest: panels not configured — run `make -C subgen/apitest test`")
	}
}

// Panel bundles a panel's API coordinates: the bootstrap creds (url+token) the test
// uses to seed inbounds and probe ground truth, plus the entity.PanelTarget the xui
// client takes.
type Panel struct {
	URL    string
	Token  string
	Target entity.PanelTarget
}

// Base is the shared API-test suite. Area suites embed it (api.Base) to get a running
// subgen server, an authenticated SDK, the two registered nodes, and the probing
// helpers below. Its exported methods/fields are the contract the area packages code
// against.
type Base struct {
	suite.Suite

	api        *Client // authenticated SDK against the spawned server
	xc         *xui.Client
	pan1, pan2 Panel
	server     *Server
	seq        int
}

// SetupSuite brings up the whole black-box stack once per embedding suite: it seeds
// the inbounds on the (docker) panels, builds + starts the subgen server, logs in, and
// registers the two nodes over the API. A failure here fails the suite fast.
func (s *Base) SetupSuite() {
	r := s.Require()

	s.pan1 = mustPanel(r, "SUBGEN_APITEST_PANEL1_URL", "SUBGEN_APITEST_PANEL1_TOKEN")
	s.pan2 = mustPanel(r, "SUBGEN_APITEST_PANEL2_URL", "SUBGEN_APITEST_PANEL2_TOKEN")

	// Seed the inbounds we need on each fresh panel (idempotent).
	r.NoError(ensureInbound(s.pan1, N1Smart, "smart"))
	r.NoError(ensureInbound(s.pan1, N1Force, "force"))
	r.NoError(ensureInbound(s.pan2, N2Smart, "smart"))
	r.NoError(ensureInbound(s.pan2, N2Force, "force"))

	s.xc = xui.New()

	// Build + start the real server, then sign in.
	s.server = StartServer(s.T())
	s.api = New(s.server.BaseURL())

	res, err := s.api.Login(AdminUser, AdminPass)
	r.NoError(err)
	r.True(res.OK, "admin login must succeed: %s", res.Message())

	// Register the fleet THROUGH THE API — this both wires the nodes for the user/sub
	// scenarios and exercises node creation end to end.
	s.RegisterNode("N1", "n1.test", s.pan1, []InboundSpec{
		{Name: "smart", Port: N1Smart},
		{Name: "force", Port: N1Force},
	})
	s.RegisterNode("N2", "n2.test", s.pan2, []InboundSpec{
		{Name: "smart", Port: N2Smart},
		{Name: "force", Port: N2Force},
	})
}

// API returns the authenticated SDK client against the spawned server.
func (s *Base) API() *Client { return s.api }

// XC returns the direct 3x-ui client used to assert panel ground truth.
func (s *Base) XC() *xui.Client { return s.xc }

// Pan1 / Pan2 return the two docker panels' coordinates.
func (s *Base) Pan1() Panel { return s.pan1 }
func (s *Base) Pan2() Panel { return s.pan2 }

// RegisterNode creates a node over POST /admin/api/nodes/save and asserts success.
func (s *Base) RegisterNode(name, vpnHost string, p Panel, inbounds []InboundSpec) {
	res, err := s.api.SaveNode(NodeSpec{
		Name: name, VPNHost: vpnHost,
		PanelBaseURL: p.URL, PanelBasePath: "/", Token: p.Token,
		Inbounds: inbounds,
	})
	s.Require().NoError(err)
	s.Require().True(res.OK, "register node %s: %s", name, res.Message())
}

// InboundID resolves a (node, inbound) pair to its node_inbounds.id via the API. This
// is what scenarios use to translate "smart on N1" into the id the user endpoints take.
func (s *Base) InboundID(node, inbound string) int64 {
	id, err := s.api.InboundID(node, inbound)
	s.Require().NoError(err)

	return id
}

// UniqueName returns a short, charset-valid, per-suite-unique name with a prefix (e.g.
// "u" for users, "n" for nodes).
func (s *Base) UniqueName(prefix string) string { s.seq++; return fmt.Sprintf("%s%d", prefix, s.seq) }

// mustPanel reads a panel's url+token from env (failing the require if absent) and
// builds its PanelTarget.
func mustPanel(r *require.Assertions, urlEnv, tokenEnv string) Panel {
	url, token := os.Getenv(urlEnv), os.Getenv(tokenEnv)
	r.NotEmpty(url, urlEnv)
	r.NotEmpty(token, tokenEnv)

	return Panel{URL: url, Token: token, Target: entity.PanelTarget{BaseURL: url, BasePath: "/", Token: token}}
}
