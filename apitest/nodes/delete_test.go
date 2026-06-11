//go:build apitest

package nodes_test

import (
	"github.com/postlog/subgen/apitest/api"
	nodeDeleteHandler "github.com/postlog/subgen/internal/handlers/node_delete"
)

// Corner cases considered for POST /admin/api/nodes/delete:
//   - success            — an unreferenced node is removed from the registry.
//   - err.blocked_by_user — an inbound bound to a user → the DB FK refuses the delete; node stays.
//   - err.blocked_by_rule — an inbound referenced by a mihomo routing rule → blocked by the FK.
//   - err.unknown_id     — deleting a non-existent node id → {ok:false}: the repo deletes no
//                          row and reports entity.ErrNodeNotFound (rows-affected, not a pre-check).

// TestDeleteSuccess covers removing an unreferenced node.
func (s *NodeSuite) TestDeleteSuccess() {
	n := s.createNode(api.NodeSpec{
		Name: s.nodeName(), VPNHost: "del.test",
		PanelBaseURL: s.Pan1().URL, PanelBasePath: "/", Token: s.Pan1().Token,
		Inbounds: []api.InboundSpec{{Name: "smart", Port: 7200}},
	})

	res, err := s.API().DeleteNode(n.ID)
	s.Require().NoError(err)
	s.Require().True(res.OK, "delete unreferenced node: %s", res.Message())

	gone, err := s.API().FindNode(n.Name)
	s.Require().NoError(err)
	s.Nil(gone, "deleted node must be gone from the registry")
}

// TestDeleteBlockedByUser covers the DB FK: a node whose inbound a user is bound to cannot
// be deleted (the FK RESTRICTs the cascade); the friendly message is returned and the node
// survives.
func (s *NodeSuite) TestDeleteBlockedByUser() {
	node := s.nodeName()
	n := s.createNode(api.NodeSpec{
		Name: node, VPNHost: "block.test",
		PanelBaseURL: s.Pan1().URL, PanelBasePath: "/", Token: s.Pan1().Token,
		Inbounds: []api.InboundSpec{{Name: "smart", Port: api.N1Smart}},
	})

	// Bind a user to the node's inbound.
	uname := s.userName()
	res, err := s.API().CreateUser(uname, []int64{s.InboundID(node, "smart")})
	s.Require().NoError(err)
	s.Require().True(res.OK, "provision user onto node: %s", res.Message())
	u, err := s.API().MustFindUser(uname)
	s.Require().NoError(err)
	s.T().Cleanup(func() { _, _ = s.API().DeleteUser(u.ID) })

	// Delete must be refused with the FK-block message; the node survives.
	del, err := s.API().DeleteNode(n.ID)
	s.Require().NoError(err)
	s.False(del.OK, "deleting a node with a bound user must be refused")
	s.Equal(nodeDeleteHandler.MsgInboundReferenced, del.Err)

	still, err := s.API().FindNode(node)
	s.Require().NoError(err)
	s.NotNil(still, "a blocked node must survive the delete attempt")
}

// TestDeleteBlockedByRule covers the other FK referent: a mihomo routing rule targeting
// the node's inbound also blocks deletion.
func (s *NodeSuite) TestDeleteBlockedByRule() {
	node := s.nodeName()
	n := s.createNode(api.NodeSpec{
		Name: node, VPNHost: "ruleblock.test",
		PanelBaseURL: s.Pan1().URL, PanelBasePath: "/", Token: s.Pan1().Token,
		Inbounds: []api.InboundSpec{{Name: "smart", Port: api.N1Smart}},
	})
	inbID := s.InboundID(node, "smart")

	// Save a config with a rule whose target is this inbound. Restore the empty config
	// after, so the reference is dropped and the node's cleanup delete can succeed.
	res, err := s.API().SaveConfig(api.Config{
		Rules: []api.ConfigRule{
			{Type: "DOMAIN-SUFFIX", Value: "ref.example", Target: api.ConfigRef{Kind: "inbound", InboundID: &inbID}},
			{Type: "MATCH", Target: api.ConfigRef{Kind: "direct"}},
		},
	})
	s.Require().NoError(err)
	s.Require().True(res.OK, "save config referencing the inbound: %s", res.Message())
	s.T().Cleanup(func() { _, _ = s.API().SaveConfig(api.Config{}) })

	// Delete must be refused while the rule references the inbound.
	del, err := s.API().DeleteNode(n.ID)
	s.Require().NoError(err)
	s.False(del.OK, "deleting a node referenced by a routing rule must be refused")
	s.Equal(nodeDeleteHandler.MsgInboundReferenced, del.Err)
}

// TestDeleteUnknownID covers deleting a never-existed node id.
func (s *NodeSuite) TestDeleteUnknownID() {
	res, err := s.API().DeleteNode(99999999)
	s.Require().NoError(err)
	s.False(res.OK, "deleting a non-existent node must report failure (no row to delete)")
	s.Equal(nodeDeleteHandler.MsgNotFound, res.Err)
}
