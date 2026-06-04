package entity

// ConnectionSelection is an admin's choice of inbounds for a user, as node_inbounds
// ids (immutable references). Any number (1+) may be selected; all inbounds are
// uniform. Maps 1:1 to the user's user_connections rows.
type ConnectionSelection struct {
	InboundIDs []int64
}

// Empty reports whether nothing was selected.
func (s ConnectionSelection) Empty() bool { return len(s.InboundIDs) == 0 }
