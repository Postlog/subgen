// Package node_save implements the nodeSave operation (POST /admin/api/nodes/save) —
// create (id 0/absent) or update a node and its inbounds. Validation and persistence live
// in the nodes service; this handler maps its errors to responses.
package node_save

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/oas"
)

// User-facing messages: name/inbound clashes (409), the field-validation sentinels the
// nodes service returns (400), and the FK refusal when an update drops a still-referenced
// inbound (400). Exported so apitest can assert against them without duplicating the text.
const (
	MsgNodeNameTaken    = "A node with this name already exists"
	MsgInboundDuplicate = "Inbound name or port already taken on this node"

	MsgNodeName          = "Node name: allowed characters are a-z, 0-9, -, space and country flags"
	MsgHost              = "VPN host address is invalid — expected a host or IP (no scheme or port)"
	MsgPanelURL          = "3x-ui base URL is invalid — expected https://host:port (no path)"
	MsgBasePath          = "Enter the panel base path (e.g. /secret/)"
	MsgNoInbounds        = "Add at least one inbound"
	MsgInboundName       = "Inbound name: allowed characters are a-z, 0-9 and -"
	MsgInboundPort       = "Inbound port must be a number between 1 and 65535"
	MsgInboundNameUq     = "Duplicate inbound name"
	MsgInboundPortUq     = "Duplicate inbound port"
	MsgInboundReferenced = "Inbound is in use — first detach users and rules from it"
)

// Handler creates or updates a node from the node form.
type Handler struct {
	svc nodesService
}

// New builds the handler.
func New(svc nodesService) *Handler { return &Handler{svc: svc} }

// NodeSave implements oas.Handler: a name/inbound clash is a 409, invalid input or a
// still-referenced dropped inbound is a 400, any unexpected (store) failure is a 500.
func (h *Handler) NodeSave(ctx context.Context, req *oas.NodeSaveReq) (oas.NodeSaveRes, error) {
	n := entity.Node{
		ID:            req.ID.Or(0),
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

	if _, err := h.svc.Save(ctx, n); err != nil {
		return h.mapErr(n.Name, err)
	}

	return &oas.MessageResponse{Message: "Node saved: " + n.Name}, nil
}

// mapErr classifies a Save failure: name/inbound clash → 409; validation / still-referenced
// inbound → 400 with a per-rule message; anything else (store) → 500.
func (h *Handler) mapErr(name string, err error) (oas.NodeSaveRes, error) {
	bad := func(msg string) (oas.NodeSaveRes, error) {
		slog.Warn("handler node_save: rejected", "name", name)
		return &oas.NodeSaveBadRequest{ErrMessage: msg}, nil
	}

	switch {
	case errors.Is(err, entity.ErrNodeNameTaken):
		slog.Warn("handler node_save: node name taken", "name", name)
		return &oas.NodeSaveConflict{ErrMessage: MsgNodeNameTaken}, nil
	case errors.Is(err, entity.ErrInboundDuplicate):
		slog.Warn("handler node_save: inbound duplicate", "name", name)
		return &oas.NodeSaveConflict{ErrMessage: MsgInboundDuplicate}, nil
	case errors.Is(err, entity.ErrInboundReferenced):
		slog.Warn("handler node_save: inbound still referenced", "name", name)
		return &oas.NodeSaveBadRequest{ErrMessage: MsgInboundReferenced}, nil
	case errors.Is(err, entity.ErrValidationNodeName):
		return bad(MsgNodeName)
	case errors.Is(err, entity.ErrValidationHost):
		return bad(MsgHost)
	case errors.Is(err, entity.ErrValidationPanelURL):
		return bad(MsgPanelURL)
	case errors.Is(err, entity.ErrValidationBasePath):
		return bad(MsgBasePath)
	case errors.Is(err, entity.ErrValidationNoInbounds):
		return bad(MsgNoInbounds)
	case errors.Is(err, entity.ErrValidationInboundName):
		return bad(MsgInboundName)
	case errors.Is(err, entity.ErrValidationInboundPort):
		return bad(MsgInboundPort)
	case errors.Is(err, entity.ErrValidationInboundNameUq):
		return bad(MsgInboundNameUq)
	case errors.Is(err, entity.ErrValidationInboundPortUq):
		return bad(MsgInboundPortUq)
	default:
		slog.Error("handler node_save: save failed", "name", name, "err", err)
		return nil, err
	}
}
