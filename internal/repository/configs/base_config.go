package configs

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/postlog/subgen/internal/entity"
)

// BaseConfigID returns the id of the base config for an engine (the one served to
// users without a custom config), or ok=false if it has not been created yet.
func (r *Repository) BaseConfigID(ctx context.Context, kind entity.ConfigKind) (int64, bool, error) {
	var id int64

	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM subscription_configs WHERE user_id IS NULL AND kind=?`, kind).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}

	if err != nil {
		return 0, false, err
	}

	return id, true, nil
}

// EnsureBaseConfigID returns the engine's base config id, creating the (empty) base
// row on first call. Used by the save path so the operator can edit the base before
// any content exists — no seed, the row appears as a side effect of the first save.
func (r *Repository) EnsureBaseConfigID(ctx context.Context, kind entity.ConfigKind) (int64, error) {
	if _, err := r.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO subscription_configs(user_id,kind,created_at) VALUES(NULL,?,?)`,
		kind, time.Now().Unix()); err != nil {
		return 0, err
	}

	var id int64
	if err := r.db.QueryRowContext(ctx,
		`SELECT id FROM subscription_configs WHERE user_id IS NULL AND kind=?`, kind).Scan(&id); err != nil {
		return 0, err
	}

	return id, nil
}
