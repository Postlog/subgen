package entity

// Panel-facing domain types. The 3x-ui client (internal/clients/xui) decodes the
// raw wire format and returns these, so the service layer never touches 3x-ui's
// JSON quirks and is easy to mock.

import "github.com/google/uuid"

// PanelInbound is one inbound on a panel, decoded.
type PanelInbound struct {
	ID      int
	Port    int
	Remark  string
	Enable  bool
	Stream  StreamInfo
	Clients []PanelClient     // settings.clients (authoritative client list)
	Stats   []PanelClientStat // clientStats (traffic + identity)
}

// PanelClient is one entry of an inbound's settings.clients.
type PanelClient struct {
	UUID  uuid.UUID
	Email string
	Flow  string
	SubID string
}

// PanelClientStat is one entry of an inbound's clientStats.
type PanelClientStat struct {
	Email  string
	UUID   uuid.UUID
	SubID  string
	Up     int64
	Down   int64
	Total  int64
	Expiry int64 // ms epoch, 0 = no expiry
	Enable bool
}

// StreamInfo is the decoded transport/security of an inbound (what a proxy needs).
type StreamInfo struct {
	Network  string // tcp | ws | grpc
	Security string // reality | tls | none

	PublicKey   string
	ShortID     string
	ServerName  string
	Fingerprint string

	SNI  string
	ALPN []string

	WSPath      string
	WSHost      string
	GRPCService string
}

// ClientSpec is a client to create on one or more inbounds.
type ClientSpec struct {
	ID    uuid.UUID // the VLESS credential
	Email string
	Flow  string
	SubID string
}

// PanelTarget is the per-call connection info for one 3x-ui panel. The xui client
// is stateless and receives this as a method argument, because different nodes use
// different credentials (creds belong on the call, not in the client).
type PanelTarget struct {
	BaseURL  string
	BasePath string
	Token    string
}
