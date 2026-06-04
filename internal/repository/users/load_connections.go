package users

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

// loadConnections fills each user's Connections via a single join query. If
// userID > 0 it loads just that user's; otherwise all.
func (r *Repository) loadConnections(ctx context.Context, byID map[int64]*entity.User, userID int64) error {
	q := `SELECT uc.id, uc.user_id, uc.inbound_id, uc.created_at,
	             ni.node_id, n.name, ni.name, ni.inbound_port
	      FROM user_connections uc
	      JOIN node_inbounds ni ON ni.id = uc.inbound_id
	      JOIN nodes n ON n.id = ni.node_id`
	args := []any{}

	if userID > 0 {
		q += ` WHERE uc.user_id = ?`

		args = append(args, userID)
	}

	q += ` ORDER BY n.name, ni.name`

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return err
	}

	defer rows.Close()

	for rows.Next() {
		var c entity.Connection
		if err := rows.Scan(&c.ID, &c.UserID, &c.InboundID, &c.CreatedAt,
			&c.NodeID, &c.Node, &c.Name, &c.Port); err != nil {
			return err
		}

		if u := byID[c.UserID]; u != nil {
			u.Connections = append(u.Connections, c)
		}
	}

	return rows.Err()
}
