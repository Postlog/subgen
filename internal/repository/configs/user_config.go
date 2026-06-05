package configs

import (
	"context"
	"database/sql"
	"errors"

	"github.com/postlog/subgen/internal/entity"
)

// UserConfigID returns the id of a user's custom config for an engine, or ok=false
// if the user has none (then the base config applies).
func (r *Repository) UserConfigID(ctx context.Context, userID int64, kind entity.ConfigKind) (int64, bool, error) {
	var id int64

	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM subscription_configs WHERE user_id=? AND kind=?`, userID, kind).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}

	if err != nil {
		return 0, false, err
	}

	return id, true, nil
}
