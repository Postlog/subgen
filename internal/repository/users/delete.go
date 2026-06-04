package users

import "context"

// Delete removes a user row (its connections cascade; panel clients are removed
// separately by the caller beforehand).
func (r *Repository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE id=?`, id)
	return err
}
