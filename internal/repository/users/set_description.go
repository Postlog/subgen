package users

import "context"

// SetDescription updates a user's free-text description (the optional admin-only note);
// a nil description clears it (stores NULL).
func (r *Repository) SetDescription(ctx context.Context, userID int64, description *string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET description=? WHERE id=?`, description, userID)
	return err
}
