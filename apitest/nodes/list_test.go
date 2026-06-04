//go:build apitest

package nodes_test

import "github.com/postlog/subgen/apitest/api"

// Corner cases considered for GET /admin/api/nodes:
//   - shape           — the baseline N1 row carries its name/host/panel URL and both
//                       inbounds (name + port), name-sorted; the token is never returned.
//   - inbound_present — cross-check that an inbound the registry lists actually exists on
//                       the real panel (nodes_api is store-only and does NOT probe the
//                       panel itself, so the ground truth comes from the direct xui read).

// TestListShape covers the registry row shape for the baseline N1 node.
func (s *NodeSuite) TestListShape() {
	n, err := s.API().MustFindNodeRow("N1")
	s.Require().NoError(err)

	s.Equal("N1", n.Name)
	s.Equal("n1.test", n.VPNHost)
	s.Equal(s.Pan1().URL, n.PanelBaseURL)
	s.Require().Len(n.Inbounds, 2, "N1 has two inbounds")

	// Inbounds are name-sorted ("force" < "smart") and carry the right ports.
	s.Equal("force", n.Inbounds[0].Name)
	s.Equal(api.N1Force, n.Inbounds[0].Port)
	s.Equal("smart", n.Inbounds[1].Name)
	s.Equal(api.N1Smart, n.Inbounds[1].Port)
}

// TestListInboundPresentOnPanel cross-checks that each inbound the registry reports for
// N1 is actually present on the real panel (the registry is store-only; this is the
// ground-truth tie-back).
func (s *NodeSuite) TestListInboundPresentOnPanel() {
	n, err := s.API().MustFindNodeRow("N1")
	s.Require().NoError(err)

	for _, in := range n.Inbounds {
		// PanelInboundID fails the test if the port is absent on the panel.
		s.Positive(s.PanelInboundID(s.Pan1(), in.Port), "inbound %q (:%d) must exist on the panel", in.Name, in.Port)
	}
}
