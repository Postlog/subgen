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
	MsgNodeNameTaken    = "Узел с таким именем уже существует"
	MsgInboundDuplicate = "Имя или порт инбаунда уже заняты на этом узле"

	MsgNodeName          = "Имя узла: разрешены a-z, 0-9, -, пробел и флаги стран"
	MsgHost              = "Адрес VPN-хоста невалиден — ожидается хост или IP (без схемы и порта)"
	MsgPanelURL          = "3x-ui base URL невалиден — ожидается https://host:port (без пути)"
	MsgBasePath          = "Укажите base path панели (например /secret/)"
	MsgNoInbounds        = "Укажите хотя бы один инбаунд"
	MsgInboundName       = "Имя инбаунда: разрешены a-z, 0-9 и -"
	MsgInboundPort       = "Порт инбаунда должен быть числом 1–65535"
	MsgInboundNameUq     = "Повторяющееся имя инбаунда"
	MsgInboundPortUq     = "Повторяющийся порт инбаунда"
	MsgInboundReferenced = "Инбаунд используется — сначала отвяжите от него пользователей и правила"
)

// Handler creates or updates a node from the node form.
type Handler struct {
	svc nodeSaver
}

// New builds the handler.
func New(svc nodeSaver) *Handler { return &Handler{svc: svc} }

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

	return &oas.MessageResponse{Message: "Узел сохранён: " + n.Name}, nil
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
