package users

import "context"

// InboundSubscribers returns, per node_inbounds.id, the sub_ids of the users connected to
// it. The fleet uses this for STATIC (hysteria2) inbounds, which are not on any panel — so
// their subscribers come from user_connections here, not from panel client stats.
func (r *Repository) InboundSubscribers(ctx context.Context) (map[int64][]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT uc.inbound_id, u.sub_id
		 FROM user_connections uc
		 JOIN users u ON u.id = uc.user_id`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	out := map[int64][]string{}

	for rows.Next() {
		var (
			inboundID int64
			subID     string
		)

		if err := rows.Scan(&inboundID, &subID); err != nil {
			return nil, err
		}

		out[inboundID] = append(out[inboundID], subID)
	}

	return out, rows.Err()
}
