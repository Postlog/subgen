package routing

import "context"

// InboundRefCounts returns, for each given node_inbounds id referenced by the mihomo
// config, how many references it has (routing rules + proxy-group members combined).
// Used to block deleting an inbound a rule or group still points at (the FK would
// also RESTRICT it; this yields a friendly pre-check message).
func (r *Repository) InboundRefCounts(ctx context.Context, inboundIDs []int64) (map[int64]int, error) {
	out := map[int64]int{}

	for _, id := range inboundIDs {
		var n int
		if err := r.db.QueryRowContext(ctx,
			`SELECT
			   (SELECT COUNT(*) FROM mihomo_routing_rules WHERE inbound_id=?) +
			   (SELECT COUNT(*) FROM mihomo_proxy_group_members WHERE inbound_id=?)`,
			id, id).Scan(&n); err != nil {
			return nil, err
		}

		if n > 0 {
			out[id] = n
		}
	}

	return out, nil
}
