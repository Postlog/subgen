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

// Inbound kinds. Both are full inbounds — access is granted the same way (user_connections,
// decided in the admin). They differ only in where the proxy PARAMS come from: VLESS (the
// default, and every pre-0005 inbound) is panel-managed — the port bridges to a real 3x-ui
// inbound and clients are provisioned there; Hysteria2 has no 3x-ui inbound (Xray-core has
// no hysteria2), so subgen renders it from the stored Hysteria2 creds and skips panel
// provisioning for it.
const (
	InboundKindVLESS     = "vless"
	InboundKindHysteria2 = "hysteria2"
)

// Inbound is one inbound on a node: a (per-node unique) name + port. Kind selects how it's
// treated. For the default VLESS kind the port bridges to the external 3x-ui inbound
// (opaque to subgen otherwise). For the Hysteria2 kind the port is the daemon's UDP listen
// port and Hysteria2 carries the client creds (identity is server-side, by daemon port →
// Xray inbound tag — see docs/hysteria2.md); it is never provisioned on a panel.
type Inbound struct {
	ID   int64  // node_inbounds.id (0 until persisted); referenced by user_connections
	Name string // ASCII letters/digits/-, unique within the node (e.g. "force")
	Port int
	Kind string // "" / "vless" (panel-managed) | "hysteria2" (static; Hysteria2 set)

	// Hysteria2 is set iff Kind == "hysteria2" — the plain hysteria2 node's creds, stored
	// in subgen (not sourced from any panel).
	Hysteria2 *Hysteria2Settings
}

// Hysteria2Settings are the stored creds for a hysteria2 inbound (rendered from here, since
// there's no 3x-ui inbound to read them from). Server = the node's VPNHost, port =
// Inbound.Port; SNI defaults to VPNHost when empty. Password is the plain hysteria2 password
// (Design A server uses type:password; no UUID). See docs/hysteria2.md.
type Hysteria2Settings struct {
	Password     string `json:"password"`
	Obfs         string `json:"obfs,omitempty"` // e.g. "salamander"
	ObfsPassword string `json:"obfs_password,omitempty"`
	SNI          string `json:"sni,omitempty"`
	Up           string `json:"up,omitempty"` // Brutal hint, e.g. "50 Mbps"
	Down         string `json:"down,omitempty"`
}

// IsHysteria2 reports whether this inbound is a hysteria2 node.
func (in Inbound) IsHysteria2() bool { return in.Kind == InboundKindHysteria2 }

// InboundLabel is an inbound's display/wire name — "<node name>-<inbound name>"
// (e.g. "🇷🇺 RU1-force"). It is unique across the fleet (node name + inbound name are
// each unique) and is used verbatim as the mihomo proxy name.
func (n Node) InboundLabel(in Inbound) string { return n.Name + "-" + in.Name }
