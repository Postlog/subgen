package routing

import (
	"context"
	"database/sql"
)

// Setting returns a config's setting value, or "" if absent.
func (r *Repository) Setting(ctx context.Context, configID int64, key string) (string, error) {
	var v string

	err := r.db.QueryRowContext(ctx, `SELECT value FROM mihomo_settings WHERE config_id=? AND key=?`, configID, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}

	return v, err
}
