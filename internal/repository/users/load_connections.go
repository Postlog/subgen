package users

import (
	"context"
	"database/sql"

	"github.com/postlog/subgen/internal/entity"
)

// loadConnections fills each user's Connections via one join query. userID>0 loads
// just that user's connections; otherwise all of them. Each variant is a complete,
// fixed query (no string-built SQL).
func (r *Repository) loadConnections(ctx context.Context, byID map[int64]*entity.User, userID int64) error {
	if userID > 0 {
		rows, err := r.db.QueryContext(ctx,
			`SELECT uc.id, uc.user_id, uc.inbound_id, uc.created_at, ni.node_id, n.name, ni.name, ni.inbound_port
			 FROM user_connections uc
			 JOIN node_inbounds ni ON ni.id = uc.inbound_id
			 JOIN nodes n ON n.id = ni.node_id
			 WHERE uc.user_id = ?
			 ORDER BY n.name, ni.name`, userID)
		if err != nil {
			return err
		}

		defer rows.Close()

		return scanConnections(rows, byID)
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT uc.id, uc.user_id, uc.inbound_id, uc.created_at, ni.node_id, n.name, ni.name, ni.inbound_port
		 FROM user_connections uc
		 JOIN node_inbounds ni ON ni.id = uc.inbound_id
		 JOIN nodes n ON n.id = ni.node_id
		 ORDER BY n.name, ni.name`)
	if err != nil {
		return err
	}

	defer rows.Close()

	return scanConnections(rows, byID)
}

// loadConnectionsForIDs fills Connections for just the given user ids — the page set
// is passed as a JSON array expanded DB-side by json_each (a single bound placeholder),
// so a page hydrates only its own users' connections. byID must hold those ids; empty
// ids is a no-op.
func (r *Repository) loadConnectionsForIDs(ctx context.Context, byID map[int64]*entity.User, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT uc.id, uc.user_id, uc.inbound_id, uc.created_at, ni.node_id, n.name, ni.name, ni.inbound_port
		 FROM user_connections uc
		 JOIN node_inbounds ni ON ni.id = uc.inbound_id
		 JOIN nodes n ON n.id = ni.node_id
		 WHERE uc.user_id IN (SELECT value FROM json_each(?))
		 ORDER BY n.name, ni.name`, idsJSON(ids))
	if err != nil {
		return err
	}

	defer rows.Close()

	return scanConnections(rows, byID)
}

// scanConnections appends each joined connection row onto its user in byID.
func scanConnections(rows *sql.Rows, byID map[int64]*entity.User) error {
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
