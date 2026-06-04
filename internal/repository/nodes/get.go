package nodes

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

// Get returns one node (with its inbounds) by id. Returns sql.ErrNoRows when the
// node does not exist.
func (r *Repository) Get(ctx context.Context, id int64) (*entity.Node, error) {
	var n entity.Node
	if err := r.db.QueryRowContext(ctx,
		`SELECT id,name,vpn_host,panel_base_url,panel_base_path,token FROM nodes WHERE id=?`, id,
	).Scan(&n.ID, &n.Name, &n.VPNHost, &n.PanelBaseURL, &n.PanelBasePath, &n.Token); err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, `SELECT id,name,inbound_port FROM node_inbounds WHERE node_id=? ORDER BY name`, id)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var in entity.Inbound
		if err := rows.Scan(&in.ID, &in.Name, &in.Port); err != nil {
			return nil, err
		}

		n.Inbounds = append(n.Inbounds, in)
	}

	return &n, rows.Err()
}
