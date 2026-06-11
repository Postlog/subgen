package nodes

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/repository/dberr"
)

// Update updates a node. The token is replaced only when setToken is true — the
// row's columns (token included) are written in a single UPDATE: when setToken is
// false the COALESCE keeps the existing token.
func (r *Repository) Update(ctx context.Context, id int64, n entity.Node, setToken bool) error {
	var token any // nil → COALESCE keeps the stored token

	if setToken {
		if n.Token == "" {
			return errors.New("token is empty")
		}

		token = n.Token
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`UPDATE nodes SET name=?,vpn_host=?,panel_base_url=?,panel_base_path=?,token=COALESCE(?,token) WHERE id=?`,
		n.Name, n.VPNHost, n.PanelBaseURL, n.PanelBasePath, token, id); err != nil {
		if dberr.IsUniqueViolation(err) {
			return entity.ErrNodeNameTaken
		}

		return err
	}
	// Inbounds are diffed by node_inbounds.id (the UI sends it back for existing
	// inbounds): id>0 is updated in place so the id stays stable across a port/name
	// change (user connections reference it); id==0 is a new inbound. Inbounds no
	// longer present are deleted first — that DELETE is RESTRICTed by user_connections
	// / mihomo, so it fails with a FOREIGN KEY violation if a removed inbound is still
	// referenced, surfaced as entity.ErrInboundReferenced.
	keep := make([]int64, 0, len(n.Inbounds))

	for _, in := range n.Inbounds {
		if in.Name != "" && in.Port != 0 && in.ID > 0 {
			keep = append(keep, in.ID)
		}
	}

	if err := deleteInboundsExcept(ctx, tx, id, keep); err != nil {
		if dberr.IsForeignKeyViolation(err) {
			return entity.ErrInboundReferenced
		}

		return err
	}

	for _, in := range n.Inbounds {
		if in.Name == "" || in.Port == 0 {
			continue
		}

		if in.ID > 0 {
			if _, err := tx.ExecContext(ctx, `UPDATE node_inbounds SET name=?,inbound_port=? WHERE id=? AND node_id=?`,
				in.Name, in.Port, in.ID, id); err != nil {
				if dberr.IsUniqueViolation(err) {
					return entity.ErrInboundDuplicate
				}

				return err
			}

			continue
		}

		if _, err := tx.ExecContext(ctx, `INSERT INTO node_inbounds(node_id,name,inbound_port) VALUES(?,?,?)`,
			id, in.Name, in.Port); err != nil {
			if dberr.IsUniqueViolation(err) {
				return entity.ErrInboundDuplicate
			}

			return err
		}
	}

	return tx.Commit()
}

// deleteInboundsExcept removes the node's inbounds whose id is not in keep. The FK
// from user_connections is RESTRICTed, so this fails if a removed inbound still has
// connections. The keep set is passed as a JSON array and expanded DB-side by
// json_each — a single bound placeholder, no string-built SQL.
func deleteInboundsExcept(ctx context.Context, tx *sql.Tx, nodeID int64, keep []int64) error {
	if len(keep) == 0 {
		_, err := tx.ExecContext(ctx, `DELETE FROM node_inbounds WHERE node_id=?`, nodeID)

		return err
	}

	keepJSON, _ := json.Marshal(keep)

	_, err := tx.ExecContext(ctx,
		`DELETE FROM node_inbounds WHERE node_id=? AND id NOT IN (SELECT value FROM json_each(?))`,
		nodeID, string(keepJSON))

	return err
}
