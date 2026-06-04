package routing

import (
	"context"
	"database/sql"
)

// Setting returns a setting value, or "" if absent.
func (r *Repository) Setting(ctx context.Context, key string) (string, error) {
	var v string

	err := r.db.QueryRowContext(ctx, `SELECT value FROM mihomo_settings WHERE key=?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}

	return v, err
}
