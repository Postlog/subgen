package users

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

// Get returns a user (with connections) by id.
func (r *Repository) Get(ctx context.Context, id int64) (*entity.User, error) {
	var u entity.User

	err := r.db.QueryRowContext(ctx, `SELECT id,name,sub_id,created_at FROM users WHERE id=?`, id).
		Scan(&u.ID, &u.Name, &u.SubID, &u.CreatedAt)
	if err != nil {
		return nil, err
	}

	if err := r.loadConnections(ctx, map[int64]*entity.User{u.ID: &u}, u.ID); err != nil {
		return nil, err
	}

	return &u, nil
}
