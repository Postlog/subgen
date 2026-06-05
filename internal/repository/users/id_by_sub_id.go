package users

import "context"

// IDBySubID resolves a subscription id to its user id. The subscription handler uses
// it after matching a token to a sub_id, to pick the user's config (custom or base).
// Returns sql.ErrNoRows if no user owns the sub_id.
func (r *Repository) IDBySubID(ctx context.Context, subID string) (int64, error) {
	var id int64

	err := r.db.QueryRowContext(ctx, `SELECT id FROM users WHERE sub_id=?`, subID).Scan(&id)
	if err != nil {
		return 0, err
	}

	return id, nil
}
