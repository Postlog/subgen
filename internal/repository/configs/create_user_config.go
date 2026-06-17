package configs

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/repository/dberr"
)

// CreateUserConfig creates a custom config for a user as a full snapshot of the
// engine's base config (clone-then-diverge). It is atomic: the anchor row and the
// cloned content commit together. Returns entity.ErrUserConfigExists if the user
// already has a custom config for this engine. When no base exists yet, the custom
// config starts empty.
func (r *Repository) CreateUserConfig(ctx context.Context, userID int64, kind entity.ConfigKind) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}

	defer tx.Rollback()

	var baseID int64

	hasBase := true

	err = tx.QueryRowContext(ctx,
		`SELECT id FROM subscription_configs WHERE user_id IS NULL AND kind=?`, kind).Scan(&baseID)
	switch {
	case err == nil:
	case errors.Is(err, sql.ErrNoRows):
		hasBase = false
	default:
		return 0, err
	}

	res, err := tx.ExecContext(ctx,
		`INSERT INTO subscription_configs(user_id,kind,created_at) VALUES(?,?,?)`,
		userID, kind, time.Now().Unix())
	if err != nil {
		if dberr.IsUniqueViolation(err) {
			return 0, entity.ErrUserConfigExists
		}

		return 0, err
	}

	newID, _ := res.LastInsertId()

	if hasBase {
		if err := r.routing.CloneConfig(ctx, tx, baseID, newID); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return newID, nil
}
