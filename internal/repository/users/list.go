package users

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

// List returns all users (with their connections), ordered by name.
func (r *Repository) List(ctx context.Context) ([]entity.User, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,name,sub_id,created_at FROM users ORDER BY name`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var out []entity.User

	for rows.Next() {
		var u entity.User
		if err := rows.Scan(&u.ID, &u.Name, &u.SubID, &u.CreatedAt); err != nil {
			return nil, err
		}

		out = append(out, u)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	byID := make(map[int64]*entity.User, len(out))
	for i := range out {
		byID[out[i].ID] = &out[i]
	}

	if err := r.loadConnections(ctx, byID, 0); err != nil {
		return nil, err
	}

	return out, nil
}
