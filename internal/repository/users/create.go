package users

import (
	"context"
	"time"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/repository/dberr"
)

// Create inserts the user and its connections (by inbound_id) in one transaction.
// ID is set on return.
func (r *Repository) Create(ctx context.Context, u *entity.User) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	now := time.Now().Unix()

	res, err := tx.ExecContext(ctx, `INSERT INTO users(name,sub_id,created_at) VALUES(?,?,?)`, u.Name, u.SubID, now)
	if err != nil {
		if dberr.IsUniqueViolation(err) {
			return entity.ErrNameTaken
		}

		return err
	}

	u.ID, _ = res.LastInsertId()
	for i := range u.Connections {
		c := &u.Connections[i]
		c.UserID = u.ID
		c.CreatedAt = now

		res, err := tx.ExecContext(ctx, `INSERT INTO user_connections(user_id,inbound_id,created_at) VALUES(?,?,?)`,
			c.UserID, c.InboundID, now)
		if err != nil {
			return err
		}

		c.ID, _ = res.LastInsertId()
	}

	return tx.Commit()
}
