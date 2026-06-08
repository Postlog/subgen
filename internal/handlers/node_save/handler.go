// Package node_save implements the nodeSave operation (POST /admin/api/nodes/save) —
// create (id 0/absent) or update a node and its inbounds.
package node_save

import (
	"context"
	"errors"
	"strings"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/handlers/web"
	"github.com/postlog/subgen/internal/oas"
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
		return &oas.NodeSaveBadRequest{ErrMessage: web.UserMessage(err)}, nil
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
				return nil, err
			}

			if msg != "" {
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
		if errors.Is(err, entity.ErrNodeNameTaken) || errors.Is(err, entity.ErrInboundDuplicate) {
			return &oas.NodeSaveConflict{ErrMessage: web.UserMessage(err)}, nil
		}

		return &oas.NodeSaveBadRequest{ErrMessage: web.UserMessage(err)}, nil
	}

	return &oas.MessageResponse{Message: "Узел сохранён: " + n.Name}, nil
}
