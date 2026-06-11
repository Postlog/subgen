// Package node_save implements the nodeSave operation (POST /admin/api/nodes/save) —
// create (id 0/absent) or update a node and its inbounds. Validation, the FK-block check
// and persistence live in the nodes service; this handler maps its errors to responses.
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

// User-facing messages: name/inbound clashes (409) and the field-validation sentinels the
// nodes service returns (400). The FK-block message is rendered by web.InboundsBlockedMessage.
const (
	msgNodeNameTaken    = "Узел с таким именем уже существует"
	msgInboundDuplicate = "Имя или порт инбаунда уже заняты на этом узле"

	msgNodeName      = "Имя узла: разрешены a-z, 0-9, -, пробел и флаги стран"
	msgHost          = "Адрес VPN-хоста невалиден — ожидается хост или IP (без схемы и порта)"
	msgPanelURL      = "3x-ui base URL невалиден — ожидается https://host:port (без пути)"
	msgBasePath      = "Укажите base path панели (например /secret/)"
	msgNoInbounds    = "Укажите хотя бы один инбаунд"
	msgInboundName   = "Имя инбаунда: разрешены a-z, 0-9 и -"
	msgInboundPort   = "Порт инбаунда должен быть числом 1–65535"
	msgInboundNameUq = "Повторяющееся имя инбаунда"
	msgInboundPortUq = "Повторяющийся порт инбаунда"
)

// Handler creates or updates a node from the node form.
type Handler struct {
	svc nodeSaver
}

// New builds the handler.
func New(svc nodeSaver) *Handler { return &Handler{svc: svc} }

// NodeSave implements oas.Handler: a name/inbound clash is a 409, invalid input or an
// FK-block is a 400, any unexpected (store) failure is a 500.
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

// mapErr classifies a Save failure: name/inbound clash → 409; validation / FK-block → 400
// with a per-rule message; anything else (store) → 500.
func (h *Handler) mapErr(name string, err error) (oas.NodeSaveRes, error) {
	var blocked entity.InboundsBlockedError

	bad := func(msg string) (oas.NodeSaveRes, error) {
		slog.Warn("handler node_save: rejected", "name", name)
		return &oas.NodeSaveBadRequest{ErrMessage: msg}, nil
	}

	switch {
	case errors.Is(err, entity.ErrNodeNameTaken):
		slog.Warn("handler node_save: node name taken", "name", name)
		return &oas.NodeSaveConflict{ErrMessage: msgNodeNameTaken}, nil
	case errors.Is(err, entity.ErrInboundDuplicate):
		slog.Warn("handler node_save: inbound duplicate", "name", name)
		return &oas.NodeSaveConflict{ErrMessage: msgInboundDuplicate}, nil
	case errors.As(err, &blocked):
		slog.Warn("handler node_save: inbound still referenced", "name", name)
		return &oas.NodeSaveBadRequest{ErrMessage: web.InboundsBlockedMessage(blocked)}, nil
	case errors.Is(err, entity.ErrValidationNodeName):
		return bad(msgNodeName)
	case errors.Is(err, entity.ErrValidationHost):
		return bad(msgHost)
	case errors.Is(err, entity.ErrValidationPanelURL):
		return bad(msgPanelURL)
	case errors.Is(err, entity.ErrValidationBasePath):
		return bad(msgBasePath)
	case errors.Is(err, entity.ErrValidationNoInbounds):
		return bad(msgNoInbounds)
	case errors.Is(err, entity.ErrValidationInboundName):
		return bad(msgInboundName)
	case errors.Is(err, entity.ErrValidationInboundPort):
		return bad(msgInboundPort)
	case errors.Is(err, entity.ErrValidationInboundNameUq):
		return bad(msgInboundNameUq)
	case errors.Is(err, entity.ErrValidationInboundPortUq):
		return bad(msgInboundPortUq)
	default:
		slog.Error("handler node_save: save failed", "name", name, "err", err)
		return nil, err
	}
}
