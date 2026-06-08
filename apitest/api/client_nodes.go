//go:build apitest

package api

import (
	"fmt"

	"github.com/postlog/subgen/internal/oas"
)

// NodeSpec/InboundSpec mirror the generated oas.NodeSaveReq save payload (their json
// tags produce the same wire body); they are restated as plain structs so the scenarios
// can build node specs with bare values rather than the generated request's Opt-wrapped
// id/token fields. Node mirrors the oas nodes-list row. DeleteNode below uses the
// generated request type directly.

// InboundSpec is one inbound row in a node save/list payload. ID==0 marks a new
// inbound; existing inbounds round-trip their node_inbounds.id so edits keep it.
type InboundSpec struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Port int    `json:"port"`
}

// NodeSpec is the node create/update payload (POST /admin/api/nodes/save). ID==0
// creates; a positive ID updates that node.
type NodeSpec struct {
	ID            int64         `json:"id"`
	Name          string        `json:"name"`
	VPNHost       string        `json:"vpnHost"`
	PanelBaseURL  string        `json:"panelBaseURL"`
	PanelBasePath string        `json:"panelBasePath"`
	Token         string        `json:"token"`
	Inbounds      []InboundSpec `json:"inbounds"`
}

// Node is one row of the nodes API (GET /admin/api/nodes). Note: the token is never
// returned (write-only secret).
type Node struct {
	ID            int64         `json:"id"`
	Name          string        `json:"name"`
	VPNHost       string        `json:"vpnHost"`
	PanelBaseURL  string        `json:"panelBaseURL"`
	PanelBasePath string        `json:"panelBasePath"`
	Inbounds      []InboundSpec `json:"inbounds"`
}

// SaveNode POSTs /admin/api/nodes/save (create or update a node). The schema marks
// inbounds as a required array whose JSON `null` (a nil Go slice) the server's decoder
// rejects, so a nil list is sent as an empty array — the handler's own "at least one
// inbound" validation then produces the friendly message.
func (c *Client) SaveNode(n NodeSpec) (Result, error) {
	if n.Inbounds == nil {
		n.Inbounds = []InboundSpec{}
	}

	return c.post("/admin/api/nodes/save", n)
}

// DeleteNode POSTs /admin/api/nodes/delete.
func (c *Client) DeleteNode(id int64) (Result, error) {
	return c.post("/admin/api/nodes/delete", oas.NodeDeleteReq{ID: id})
}

// ListNodes GETs /admin/api/nodes and returns the registry rows.
func (c *Client) ListNodes() ([]Node, error) {
	var resp struct {
		Nodes []Node `json:"nodes"`
	}

	if err := c.getJSON("/admin/api/nodes", &resp); err != nil {
		return nil, err
	}

	return resp.Nodes, nil
}

// FindNode GETs the nodes list and returns the row whose name matches, or nil.
func (c *Client) FindNode(name string) (*Node, error) {
	nodes, err := c.ListNodes()
	if err != nil {
		return nil, err
	}

	for i := range nodes {
		if nodes[i].Name == name {
			return &nodes[i], nil
		}
	}

	return nil, nil
}

// MustFindNodeRow is FindNode that errors if the node is absent (the common case for
// the registered baseline).
func (c *Client) MustFindNodeRow(name string) (*Node, error) {
	n, err := c.FindNode(name)
	if err != nil {
		return nil, err
	}

	if n == nil {
		return nil, fmt.Errorf("node %q not found in nodes list", name)
	}

	return n, nil
}

// InboundID returns the node_inbounds.id of the named inbound on the named node,
// resolved through the nodes API. This is the canonical id the user/config endpoints
// take, so it is how scenarios translate "smart on N1" into a wire request.
func (c *Client) InboundID(node, inbound string) (int64, error) {
	n, err := c.FindNode(node)
	if err != nil {
		return 0, err
	}

	if n == nil {
		return 0, fmt.Errorf("node %q not found", node)
	}

	for _, in := range n.Inbounds {
		if in.Name == inbound {
			return in.ID, nil
		}
	}

	return 0, fmt.Errorf("node %q has no inbound %q", node, inbound)
}
