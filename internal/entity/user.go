package entity

// User is a service-owned subscriber: a nickname + a subscription id, plus one
// Connection per inbound it is provisioned on.
type User struct {
	ID          int64
	Name        string  // admin nickname (unique)
	SubID       string  // subscription id (shared across all the user's clients)
	Description *string // optional free-text note, shown only in the admin UI; nil = unset
	CreatedAt   int64
	Connections []Connection
}

// UserCreateParams is the input to provisioning.CreateUser: the chosen nickname, the
// inbound ids to bind and an optional free-text description (nil = none).
type UserCreateParams struct {
	Name        string
	Description *string
	InboundIDs  []int64
}

// UserEditParams is the input to provisioning.EditUser: the target user id, the new
// inbound ids and the optional description (nil = cleared).
type UserEditParams struct {
	ID          int64
	Description *string
	InboundIDs  []int64
}

// UserListParams selects and pages the users list at the repository level (so the
// store never materialises every user). NameQuery is a case-insensitive substring
// match on the nickname; InboundIDs is an OR-filter (the user has at least one of
// these node_inbounds.id connections). A nil/empty filter field means "no filter".
type UserListParams struct {
	NameQuery  string
	InboundIDs []int64
	Limit      int
	Offset     int
}

// UserPage is one page of users plus the total count matching the (unpaged) filter.
type UserPage struct {
	Users []User
	Total int64
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
