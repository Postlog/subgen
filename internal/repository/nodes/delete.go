package nodes

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/repository/dberr"
)

// Delete removes a node and (via cascade) its inbounds. The cascade is RESTRICTed by the
// user_connections / mihomo FKs onto node_inbounds, so it fails with a FOREIGN KEY
// violation when any inbound is still referenced — surfaced as entity.ErrInboundReferenced
// (the FK is the source of truth; no pre-check SELECT). Deleting an id that isn't there
// removes nothing and returns entity.ErrNodeNotFound (reported from rows-affected, not a
// pre-check).
func (r *Repository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM nodes WHERE id=?`, id)
	if err != nil {
		if dberr.IsForeignKeyViolation(err) {
			return entity.ErrInboundReferenced
		}

		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if n == 0 {
		return entity.ErrNodeNotFound
	}

	return nil
}
