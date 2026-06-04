package nodes

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/repository/dberr"
)

// Create inserts a node and its inbounds.
func (r *Repository) Create(ctx context.Context, n entity.Node) (int64, error) {
	if n.Token == "" {
		return 0, errors.New("token is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}

	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `INSERT INTO nodes(name,vpn_host,panel_base_url,panel_base_path,token,created_at) VALUES(?,?,?,?,?,?)`,
		n.Name, n.VPNHost, n.PanelBaseURL, n.PanelBasePath, n.Token, time.Now().Unix())
	if err != nil {
		if dberr.IsUniqueViolation(err) {
			return 0, entity.ErrNodeNameTaken
		}

		return 0, err
	}

	id, _ := res.LastInsertId()
	if err := insertInbounds(ctx, tx, id, n.Inbounds); err != nil {
		return 0, err
	}

	return id, tx.Commit()
}

func insertInbounds(ctx context.Context, tx *sql.Tx, nodeID int64, inbounds []entity.Inbound) error {
	for _, in := range inbounds {
		if in.Name == "" || in.Port == 0 {
			continue
		}

		if _, err := tx.ExecContext(ctx, `INSERT INTO node_inbounds(node_id,name,inbound_port) VALUES(?,?,?)`, nodeID, in.Name, in.Port); err != nil {
			if dberr.IsUniqueViolation(err) {
				return entity.ErrInboundDuplicate
			}

			return err
		}
	}

	return nil
}
