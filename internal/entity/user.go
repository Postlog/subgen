package entity

// User is a service-owned subscriber: a nickname + a subscription id, plus one
// Connection per inbound it is provisioned on.
type User struct {
	ID          int64
	Name        string // admin nickname (unique)
	SubID       string // subscription id (shared across all the user's clients)
	CreatedAt   int64
	Connections []Connection
}

// Connection is one (user, inbound) binding. Node/Name/Port are resolved by join
// from node_inbounds; the client email/subId are the user's, not per-connection.
// The inbound label (for display / wire-naming) is Node + "-" + Name.
type Connection struct {
	ID        int64
	UserID    int64
	InboundID int64 // node_inbounds.id — the canonical reference
	CreatedAt int64
	NodeID    int64  // node id (joined) — used for lookups/keying
	Node      string // node name (joined) — display / wire-naming
	Name      string // inbound name (joined) — display / wire-naming
	Port      int    // inbound port (joined) — bridge to the 3x-ui inbound
}
