package users

import (
	"context"
	"database/sql"
	"strings"

	"github.com/postlog/subgen/internal/entity"
)

// connSelect is the shared join projecting a connection plus its inbound's node /
// name / port (for display and wire-naming). Callers append their own WHERE/ORDER.
const connSelect = `SELECT uc.id, uc.user_id, uc.inbound_id, uc.created_at,
	             ni.node_id, n.name, ni.name, ni.inbound_port
	      FROM user_connections uc
	      JOIN node_inbounds ni ON ni.id = uc.inbound_id
	      JOIN nodes n ON n.id = ni.node_id`

// loadConnections fills each user's Connections via a single join query. If
// userID > 0 it loads just that user's; otherwise all.
func (r *Repository) loadConnections(ctx context.Context, byID map[int64]*entity.User, userID int64) error {
	q := connSelect
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

	return scanConnections(rows, byID)
}

// loadConnectionsForIDs fills Connections for just the given user ids via one join
// (WHERE uc.user_id IN (...)) — so a page hydrates only its own users' connections.
// byID must hold those ids; empty ids is a no-op.
func (r *Repository) loadConnectionsForIDs(ctx context.Context, byID map[int64]*entity.User, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	ph := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	//nolint:gosec // G202: only "?" placeholders are concatenated; the ids are bound via args.
	q := connSelect + ` WHERE uc.user_id IN (` + ph + `) ORDER BY n.name, ni.name`

	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	rows, err := r.db.QueryContext(ctx, q, args...)
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
