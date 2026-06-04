package entity

// Node is a fleet node: a VPN host + its 3x-ui panel + the inbounds it exposes.
type Node struct {
	ID   int64
	Name string // display/wire name; allows ASCII letters/digits/-/space + country flags (e.g. "🇷🇺 RU1")
	// VPNHost is what clients dial (server= in the proxy).
	VPNHost string
	// PanelBaseURL/PanelBasePath stay strings (not *url.URL): they round-trip
	// through SQLite text and HTML forms and are only joined with the API path —
	// net/url.URL (a mutable struct, verbose to (un)marshal) buys nothing here.
	// Format is validated at the handler boundary (web.ValidateNode).
	PanelBaseURL  string
	PanelBasePath string
	Token         string // 3x-ui API Bearer token (write-only in the UI)
	Inbounds      []Inbound
}

// Inbound is one panel inbound on a node: a (per-node unique) name + the port that
// bridges to the external 3x-ui inbound. There is no inbound "type" — every inbound
// is uniform; whatever an inbound does on the node side (selective routing, a plain
// exit, …) is a node-side Xray detail, opaque to subgen.
type Inbound struct {
	ID   int64  // node_inbounds.id (0 until persisted); referenced by user_connections
	Name string // ASCII letters/digits/-, unique within the node (e.g. "force")
	Port int
}

// InboundLabel is an inbound's display/wire name — "<node name>-<inbound name>"
// (e.g. "🇷🇺 RU1-force"). It is unique across the fleet (node name + inbound name are
// each unique) and is used verbatim as the mihomo proxy name.
func (n Node) InboundLabel(in Inbound) string { return n.Name + "-" + in.Name }
