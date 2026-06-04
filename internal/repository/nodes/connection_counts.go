package nodes

import "context"

// ConnectionCountsByInbound returns, for each given node_inbounds id that has at
// least one user connection, the number of connections. Used to block deleting an
// inbound/node that users still rely on (would orphan the 3x-ui client).
func (r *Repository) ConnectionCountsByInbound(ctx context.Context, inboundIDs []int64) (map[int64]int, error) {
	out := map[int64]int{}

	for _, id := range inboundIDs {
		var n int
		if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_connections WHERE inbound_id=?`, id).Scan(&n); err != nil {
			return nil, err
		}

		if n > 0 {
			out[id] = n
		}
	}

	return out, nil
}
