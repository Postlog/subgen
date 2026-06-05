package configs

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

// UserConfigUserIDs returns the ids of users that have a custom config for an engine,
// ordered by id. The admin scope selector uses it to list editable custom configs.
func (r *Repository) UserConfigUserIDs(ctx context.Context, kind entity.ConfigKind) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT user_id FROM subscription_configs WHERE user_id IS NOT NULL AND kind=? ORDER BY user_id`, kind)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var out []int64

	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}

		out = append(out, id)
	}

	return out, rows.Err()
}
