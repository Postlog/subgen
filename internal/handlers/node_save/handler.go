// Package node_save implements the nodeSave operation (POST /admin/api/nodes/save) —
// create (id 0/absent) or update a node and its inbounds.
package node_save

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/handlers/web"
	"github.com/postlog/subgen/internal/oas"
)

// User-facing messages for the domain conflicts the node repo can return. Field-level
// validation messages are produced by web.ValidateNode (interpolating the offending
// value), and the FK-block message by web.InboundsBlocking.
const (
	msgNodeNameTaken    = "Узел с таким именем уже существует"
	msgInboundDuplicate = "Имя или порт инбаунда уже заняты на этом узле"
)

// Handler creates or updates a node from the node form.
type Handler struct {
	nodes   nodeRepo
	routing routingRepo
}

// New builds the handler.
func New(nodes nodeRepo, routing routingRepo) *Handler {
	return &Handler{nodes: nodes, routing: routing}
}

// NodeSave implements oas.Handler.
func (h *Handler) NodeSave(ctx context.Context, req *oas.NodeSaveReq) (oas.NodeSaveRes, error) {
	n := entity.Node{
		Name:          strings.TrimSpace(req.Name),
		VPNHost:       strings.TrimSpace(req.VpnHost),
		PanelBaseURL:  strings.TrimSpace(req.PanelBaseURL),
		PanelBasePath: strings.TrimSpace(req.PanelBasePath),
		Token:         strings.TrimSpace(req.Token.Or("")),
	}

	for _, in := range req.Inbounds {
		if in.Port == 0 {
			continue // blank inbound row
		}

		n.Inbounds = append(n.Inbounds, entity.Inbound{ID: in.ID.Or(0), Name: strings.TrimSpace(in.Name), Port: in.Port})
	}

	if err := web.ValidateNode(&n); err != nil {
		slog.Warn("handler node_save: invalid node", "name", n.Name, "err", err)
		return &oas.NodeSaveBadRequest{ErrMessage: err.Error()}, nil
	}

	id := req.ID.Or(0)

	// On update, block removing an inbound that is still referenced (users / mihomo
	// rules / group members). "removed" = current ids absent from the submission.
	if id > 0 {
		kept := map[int64]bool{}

		for _, in := range n.Inbounds {
			if in.ID > 0 {
				kept[in.ID] = true
			}
		}

		var removed []int64

		if cur, err := h.nodes.Get(ctx, id); err == nil {
			for _, in := range cur.Inbounds {
				if !kept[in.ID] {
					removed = append(removed, in.ID)
				}
			}
		}

		if len(removed) > 0 {
			msg, err := web.InboundsBlocking(ctx, h.nodes, h.routing, id, removed)
			if err != nil {
				slog.Error("handler node_save: inbound-block check failed", "id", id, "err", err)
				return nil, err
			}

			if msg != "" {
				slog.Warn("handler node_save: inbound still referenced", "id", id)
				return &oas.NodeSaveBadRequest{ErrMessage: msg}, nil
			}
		}
	}

	var err error
	if id > 0 {
		err = h.nodes.Update(ctx, id, n, n.Token != "")
	} else {
		_, err = h.nodes.Create(ctx, n)
	}

	if err != nil {
		switch {
		case errors.Is(err, entity.ErrNodeNameTaken):
			slog.Warn("handler node_save: node name taken", "name", n.Name)
			return &oas.NodeSaveConflict{ErrMessage: msgNodeNameTaken}, nil
		case errors.Is(err, entity.ErrInboundDuplicate):
			slog.Warn("handler node_save: inbound duplicate", "name", n.Name)
			return &oas.NodeSaveConflict{ErrMessage: msgInboundDuplicate}, nil
		default:
			slog.Error("handler node_save: save failed", "name", n.Name, "id", id, "err", err)
			return nil, err
		}
	}

	return &oas.MessageResponse{Message: "Узел сохранён: " + n.Name}, nil
}
