package nodes

import "context"

// Delete removes a node and (via cascade) its inbounds.
func (r *Repository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM nodes WHERE id=?`, id)
	return err
}
