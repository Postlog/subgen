//go:build apitest

package nodes_test

import (
	"github.com/postlog/subgen/apitest/api"
	nodeSaveHandler "github.com/postlog/subgen/internal/handlers/node_save"
)

// Corner cases considered for POST /admin/api/nodes/save:
//   - create                — a new node with one inbound appears in the registry.
//   - update.add_inbound     — add a second inbound + change a field; kept inbound keeps its id.
//   - update.keep_token      — update with an EMPTY token preserves the stored token
//                              (verified end-to-end: a user still provisions onto the panel).
//   - update.replace_token   — update with a NEW token replaces it (the node still works).
//   - err.bad_vpn_host       — host with a scheme/port → rejected ("invalid").
//   - err.no_inbounds        — zero inbounds → rejected ("at least one inbound").
//   - err.bad_inbound_name   — inbound name with illegal chars → rejected ("inbound name").
//   - err.bad_node_name      — node name with illegal chars → rejected ("node name").
//   - err.duplicate_node_name — second node with an existing name → "A node ... already exists".
//   - err.duplicate_inbound_name — two inbounds same name in one payload → "Duplicate inbound name"
//                              (web.ValidateNode catches the in-payload dup before the DB).
//   - err.duplicate_inbound_port — two inbounds same port in one payload → "Duplicate inbound port".
//   - err.malformed_json     — non-JSON body → MsgBadRequest.

// TestSaveCreateUpdate covers the create + update happy paths against the registry.
func (s *NodeSuite) TestSaveCreateUpdate() {
	name := s.nodeName()

	created := s.createNode(api.NodeSpec{
		Name: name, VPNHost: "crud.test",
		PanelBaseURL: s.Pan1().URL, PanelBasePath: "/", Token: s.Pan1().Token,
		Inbounds: []api.InboundSpec{{Name: "smart", Port: 7001}},
	})
	s.Require().Len(created.Inbounds, 1)
	s.Equal("crud.test", created.VPNHost)
	s.Equal(7001, created.Inbounds[0].Port)

	// Update: change the VPN host and add a second inbound; existing inbound carries its
	// id back so it stays stable.
	res, err := s.API().SaveNode(api.NodeSpec{
		ID: created.ID, Name: name, VPNHost: "crud2.test",
		PanelBaseURL: s.Pan1().URL, PanelBasePath: "/", Token: s.Pan1().Token,
		Inbounds: []api.InboundSpec{
			{ID: created.Inbounds[0].ID, Name: "smart", Port: 7001},
			{Name: "force", Port: 7002},
		},
	})
	s.Require().NoError(err)
	s.Require().True(res.OK, "update node: %s", res.Message())

	updated, err := s.API().FindNode(name)
	s.Require().NoError(err)
	s.Require().NotNil(updated)
	s.Equal("crud2.test", updated.VPNHost, "VPN host must be updated")
	s.Require().Len(updated.Inbounds, 2, "second inbound must be added")
	s.Equal(created.Inbounds[0].ID, inboundIDByName(updated, "smart"), "kept inbound must keep its id")
}

// TestSaveTokenHandling covers the keep-token (empty token on update) and replace-token
// paths, verified end-to-end: a node whose token is preserved/replaced still provisions
// a real client onto the panel. The throwaway node points at panel1's smart port, so a
// user bound to it lands on that inbound.
func (s *NodeSuite) TestSaveTokenHandling() {
	s.Run("keep_token", func() {
		name := s.nodeName()
		n := s.createNode(api.NodeSpec{
			Name: name, VPNHost: "tok.test",
			PanelBaseURL: s.Pan1().URL, PanelBasePath: "/", Token: s.Pan1().Token,
			Inbounds: []api.InboundSpec{{Name: "smart", Port: api.N1Smart}},
		})

		// Update with an EMPTY token (and a changed host) — the stored token must persist.
		res, err := s.API().SaveNode(api.NodeSpec{
			ID: n.ID, Name: name, VPNHost: "tok2.test",
			PanelBaseURL: s.Pan1().URL, PanelBasePath: "/", Token: "",
			Inbounds: []api.InboundSpec{{ID: n.Inbounds[0].ID, Name: "smart", Port: api.N1Smart}},
		})
		s.Require().NoError(err)
		s.Require().True(res.OK, "update with empty token must succeed (keep token): %s", res.Message())

		// End-to-end proof the token survived: provision a user onto this node.
		s.provisionProbe(name)
	})

	s.Run("replace_token", func() {
		name := s.nodeName()
		n := s.createNode(api.NodeSpec{
			Name: name, VPNHost: "rep.test",
			PanelBaseURL: s.Pan1().URL, PanelBasePath: "/", Token: "bogus-will-be-replaced",
			Inbounds: []api.InboundSpec{{Name: "smart", Port: api.N1Smart}},
		})

		// Replace with the real token — provisioning must now work.
		res, err := s.API().SaveNode(api.NodeSpec{
			ID: n.ID, Name: name, VPNHost: "rep.test",
			PanelBaseURL: s.Pan1().URL, PanelBasePath: "/", Token: s.Pan1().Token,
			Inbounds: []api.InboundSpec{{ID: n.Inbounds[0].ID, Name: "smart", Port: api.N1Smart}},
		})
		s.Require().NoError(err)
		s.Require().True(res.OK, "update with a new token must succeed: %s", res.Message())

		s.provisionProbe(name)
	})
}

// provisionProbe creates a user bound to the named node's smart inbound and asserts the
// client lands on panel1's smart inbound — proving the node's stored token is usable.
func (s *NodeSuite) provisionProbe(node string) {
	uname := s.userName()
	res, err := s.API().CreateUser(uname, []int64{s.InboundID(node, "smart")})
	s.Require().NoError(err)
	s.Require().True(res.OK, "provision onto %s must succeed (token usable): %s", node, res.Message())

	u, err := s.API().MustFindUser(uname)
	s.Require().NoError(err)
	s.T().Cleanup(func() { _, _ = s.API().DeleteUser(u.ID) })

	s.RequireClient(s.Pan1(), api.N1Smart, uname)
}

// TestSaveValidation covers every rejected save with its exact/substring friendly text.
func (s *NodeSuite) TestSaveValidation() {
	valid := func() api.NodeSpec {
		return api.NodeSpec{
			Name: s.nodeName(), VPNHost: "ok.test",
			PanelBaseURL: s.Pan1().URL, PanelBasePath: "/", Token: s.Pan1().Token,
			Inbounds: []api.InboundSpec{{Name: "smart", Port: 7100}},
		}
	}

	s.Run("bad_vpn_host", func() {
		spec := valid()
		spec.VPNHost = "http://bad:8080"
		s.rejected(spec, nodeSaveHandler.MsgHost)
	})

	s.Run("no_inbounds", func() {
		spec := valid()
		spec.Inbounds = nil
		s.rejected(spec, nodeSaveHandler.MsgNoInbounds)
	})

	s.Run("bad_inbound_name", func() {
		spec := valid()
		spec.Inbounds = []api.InboundSpec{{Name: "bad name!", Port: 7100}}
		s.rejected(spec, nodeSaveHandler.MsgInboundName)
	})

	s.Run("bad_node_name", func() {
		spec := valid()
		spec.Name = "bad/name"
		s.rejected(spec, nodeSaveHandler.MsgNodeName)
	})

	s.Run("duplicate_inbound_name", func() {
		spec := valid()
		spec.Inbounds = []api.InboundSpec{
			{Name: "dup", Port: 7101},
			{Name: "dup", Port: 7102},
		}
		s.rejected(spec, nodeSaveHandler.MsgInboundNameUq)
	})

	s.Run("duplicate_inbound_port", func() {
		spec := valid()
		spec.Inbounds = []api.InboundSpec{
			{Name: "a", Port: 7103},
			{Name: "b", Port: 7103},
		}
		s.rejected(spec, nodeSaveHandler.MsgInboundPortUq)
	})

	s.Run("duplicate_node_name", func() {
		// Create one, then a second with the same name → DB-level node-name conflict.
		first := s.createNode(valid())

		dup := valid()
		dup.Name = first.Name
		res, err := s.API().SaveNode(dup)
		s.Require().NoError(err)
		s.False(res.OK, "duplicate node name must be rejected")
		s.Equal(nodeSaveHandler.MsgNodeNameTaken, res.Err)
	})

	s.Run("malformed_json", func() {
		resp, err := s.API().PostRaw("/admin/api/nodes/save", "application/json", []byte("{bad"))
		s.Require().NoError(err)
		decoded, err := api.DecodeResult(resp.Body)
		s.Require().NoError(err)
		s.False(decoded.OK)
		s.Equal(api.MsgBadRequest, decoded.Err)
	})
}

// rejected POSTs a node spec that must be rejected and asserts {ok:false} carrying exactly
// the expected friendly message (imported from the handler, not restated here).
func (s *NodeSuite) rejected(spec api.NodeSpec, wantMsg string) {
	res, err := s.API().SaveNode(spec)
	s.Require().NoError(err)
	s.Require().False(res.OK, "node must be rejected (wanted %q)", wantMsg)
	s.Equal(wantMsg, res.Err)
}
