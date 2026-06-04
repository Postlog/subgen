package nodes

import (
	"context"

	"github.com/postlog/subgen/internal/entity"
)

// List returns all nodes with their inbounds, ordered by name.
func (r *Repository) List(ctx context.Context) ([]entity.Node, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,name,vpn_host,panel_base_url,panel_base_path,token FROM nodes ORDER BY name`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var nodes []entity.Node

	byID := map[int64]*entity.Node{}

	for rows.Next() {
		var n entity.Node
		if err := rows.Scan(&n.ID, &n.Name, &n.VPNHost, &n.PanelBaseURL, &n.PanelBasePath, &n.Token); err != nil {
			return nil, err
		}

		nodes = append(nodes, n)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range nodes {
		byID[nodes[i].ID] = &nodes[i]
	}

	inb, err := r.db.QueryContext(ctx, `SELECT id,node_id,name,inbound_port FROM node_inbounds`)
	if err != nil {
		return nil, err
	}

	defer inb.Close()

	for inb.Next() {
		var nid int64

		var in entity.Inbound
		if err := inb.Scan(&in.ID, &nid, &in.Name, &in.Port); err != nil {
			return nil, err
		}

		if n := byID[nid]; n != nil {
			n.Inbounds = append(n.Inbounds, in)
		}
	}

	return nodes, inb.Err()
}
