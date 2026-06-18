// Package user_create implements the userCreate operation (POST /admin/api/users/create).
package user_create

import (
	"context"
	"errors"
	"log/slog"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/oas"
)

// User-facing messages. Exported so apitest can assert against them without duplicating
// the text.
const (
	MsgCreated         = "User created"
	MsgInvalidName     = "Client name: allowed characters are a-z, 0-9, _ and -. From 1 to 32 characters"
	MsgNameTaken       = "Name already taken"
	MsgNoConnection    = "Select at least one connection"
	MsgInboundNotFound = "The specified inbound was not found"
	MsgNodeNotFound    = "Node not found"
	MsgDescTooLong     = "Description is too long (max 500 characters)"
)

// Handler provisions a new user.
type Handler struct {
	svc provisioningService
}

// New builds the handler.
func New(svc provisioningService) *Handler { return &Handler{svc: svc} }

// UserCreate implements oas.Handler: a nickname clash (or a panel client clash) is a
// 409, other invalid input is a 400, and any unexpected (infra) failure is a 500.
func (h *Handler) UserCreate(ctx context.Context, req *oas.UserCreateReq) (oas.UserCreateRes, error) {
	var desc *string
	if v, ok := req.Description.Get(); ok {
		desc = &v
	}

	_, err := h.svc.CreateUser(ctx, entity.UserCreateParams{
		Name:        req.Name,
		Description: desc,
		InboundIDs:  req.InboundIDs,
	})
	if err == nil {
		return &oas.MessageResponse{Message: MsgCreated}, nil
	}

	var pce entity.PanelClientExistsError

	switch {
	case errors.Is(err, entity.ErrNameTaken):
		slog.Warn("handler user_create: name taken", "name", req.Name)
		return &oas.UserCreateConflict{ErrMessage: MsgNameTaken}, nil
	case errors.As(err, &pce):
		slog.Warn("handler user_create: email exists on panel", "name", req.Name, "node", pce.Node)
		return &oas.UserCreateConflict{ErrMessage: "panel \"" + pce.Node + "\" already has a client with this name — delete it there manually or pick another name"}, nil
	case errors.Is(err, entity.ErrInvalidUserName):
		slog.Warn("handler user_create: invalid name", "name", req.Name)
		return &oas.UserCreateBadRequest{ErrMessage: MsgInvalidName}, nil
	case errors.Is(err, entity.ErrNoConnectionSelected):
		slog.Warn("handler user_create: no connection selected", "name", req.Name)
		return &oas.UserCreateBadRequest{ErrMessage: MsgNoConnection}, nil
	case errors.Is(err, entity.ErrDescriptionTooLong):
		slog.Warn("handler user_create: description too long", "name", req.Name)
		return &oas.UserCreateBadRequest{ErrMessage: MsgDescTooLong}, nil
	case errors.Is(err, entity.ErrInboundNotFound):
		slog.Warn("handler user_create: inbound not found", "name", req.Name)
		return &oas.UserCreateBadRequest{ErrMessage: MsgInboundNotFound}, nil
	case errors.Is(err, entity.ErrNodeNotFound):
		slog.Warn("handler user_create: node not found", "name", req.Name)
		return &oas.UserCreateBadRequest{ErrMessage: MsgNodeNotFound}, nil
	default:
		slog.Error("handler user_create: create failed", "name", req.Name, "err", err)
		return nil, err
	}
}
