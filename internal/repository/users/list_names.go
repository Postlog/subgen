package users

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

// ListNames returns every user as id + name only (no connections), ordered by name —
// a cheap lookup for resolving ids to display names and for the config scope picker,
// without hydrating the full connection join.
func (r *Repository) ListNames(ctx context.Context) ([]entity.User, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,name FROM users ORDER BY name`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var out []entity.User

	for rows.Next() {
		var u entity.User
		if err := rows.Scan(&u.ID, &u.Name); err != nil {
			return nil, err
		}

		out = append(out, u)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}
