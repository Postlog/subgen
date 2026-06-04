package users

import (
	"context"
	"time"
)

// ReplaceConnections sets a user's connections to exactly the given inbound ids
// (insert missing, delete removed) in one transaction.
func (r *Repository) ReplaceConnections(ctx context.Context, userID int64, inboundIDs []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	want := map[int64]bool{}
	for _, id := range inboundIDs {
		want[id] = true
	}

	rows, err := tx.QueryContext(ctx, `SELECT id,inbound_id FROM user_connections WHERE user_id=?`, userID)
	if err != nil {
		return err
	}

	have := map[int64]int64{} // inbound_id -> row id

	for rows.Next() {
		var rowID, inbID int64
		if err := rows.Scan(&rowID, &inbID); err != nil {
			rows.Close()
			return err
		}

		have[inbID] = rowID
	}

	rows.Close()

	now := time.Now().Unix()

	for inbID := range want {
		if _, ok := have[inbID]; !ok {
			if _, err := tx.ExecContext(ctx, `INSERT INTO user_connections(user_id,inbound_id,created_at) VALUES(?,?,?)`, userID, inbID, now); err != nil {
				return err
			}
		}
	}

	for inbID, rowID := range have {
		if !want[inbID] {
			if _, err := tx.ExecContext(ctx, `DELETE FROM user_connections WHERE id=?`, rowID); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}
