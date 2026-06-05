package configs

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

// DeleteUserConfig removes a user's custom config for an engine; its engine content
// cascades away, and the user's subscription falls back to the base config. Returns
// entity.ErrUserConfigNotFound if the user had no custom config.
func (r *Repository) DeleteUserConfig(ctx context.Context, userID int64, kind entity.ConfigKind) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM subscription_configs WHERE user_id=? AND kind=?`, userID, kind)
	if err != nil {
		return err
	}

	if n, _ := res.RowsAffected(); n == 0 {
		return entity.ErrUserConfigNotFound
	}

	return nil
}
