//go:build apitest

// Package nodes_test drives subgen's node endpoints over the real HTTP API: save
// (create/update), delete (with FK-block pre-checks), and the nodes read API. Node
// state lives in subgen's own SQLite store — node CRUD performs no panel mutation — but
// the suite embeds api.Base (which registers the N1/N2 baseline fleet against the docker
// panels in SetupSuite), so it is gated on the panels being configured. The throwaway
// nodes these scenarios create/delete don't disturb that baseline.
//
// One Test* per endpoint/scenario; corner cases are dotted subtests.
package nodes_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/postlog/subgen/apitest/api"
)

// NodeSuite covers the node endpoints.
type NodeSuite struct{ api.Base }

func TestNodeSuite(t *testing.T) {
	api.SkipUnlessConfigured(t)
	suite.Run(t, new(NodeSuite))
}

// nodeName returns a unique, charset-valid node name for a scenario.
func (s *NodeSuite) nodeName() string { return s.UniqueName("n") }

// userName returns a unique, charset-valid user nickname (for the FK-block scenario).
func (s *NodeSuite) userName() string { return s.UniqueName("nu") }

// createNode saves a throwaway node over the API, asserts {ok}, reads the row back for
// its id, and schedules its deletion at the end of the scenario.
func (s *NodeSuite) createNode(spec api.NodeSpec) *api.Node {
	res, err := s.API().SaveNode(spec)
	s.Require().NoError(err)
	s.Require().True(res.OK, "SaveNode(%q): %s", spec.Name, res.Message())

	n, err := s.API().FindNode(spec.Name)
	s.Require().NoError(err)
	s.Require().NotNil(n, "created node %q must appear in the nodes list", spec.Name)

	s.T().Cleanup(func() { _, _ = s.API().DeleteNode(n.ID) })

	return n
}

// inboundIDByName returns the id of the named inbound on a node row, or 0.
func inboundIDByName(n *api.Node, name string) int64 {
	for _, in := range n.Inbounds {
		if in.Name == name {
			return in.ID
		}
	}

	return 0
}
