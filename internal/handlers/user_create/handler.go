// Package user_create implements the userCreate operation (POST /admin/api/users/create).
package user_create

import (
	"context"
	"errors"
	"log/slog"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/oas"
)

// User-facing messages for the domain errors CreateUser can return.
const (
	msgInvalidName     = "Имя клиента: разрешены символы a-z, 0-9, _ и -. От 1 до 32 символов"
	msgNameTaken       = "Имя занято"
	msgNoConnection    = "Выберите хотя бы одно подключение"
	msgInboundNotFound = "Указанный инбаунд не найден"
	msgNodeNotFound    = "Узел не найден"
)

// Handler provisions a new user.
type Handler struct {
	svc creator
}

// New builds the handler.
func New(svc creator) *Handler { return &Handler{svc: svc} }

// UserCreate implements oas.Handler: a nickname clash (or a panel client clash) is a
// 409, other invalid input is a 400, and any unexpected (infra) failure is a 500.
func (h *Handler) UserCreate(ctx context.Context, req *oas.UserCreateReq) (oas.UserCreateRes, error) {
	_, err := h.svc.CreateUser(ctx, req.Name, entity.ConnectionSelection{InboundIDs: req.InboundIDs})
	if err == nil {
		return &oas.MessageResponse{Message: "Создан пользователь"}, nil
	}

	var pce entity.PanelClientExistsError

	switch {
	case errors.Is(err, entity.ErrNameTaken):
		slog.Warn("handler user_create: name taken", "name", req.Name)
		return &oas.UserCreateConflict{ErrMessage: msgNameTaken}, nil
	case errors.As(err, &pce):
		slog.Warn("handler user_create: email exists on panel", "name", req.Name, "node", pce.Node)
		return &oas.UserCreateConflict{ErrMessage: "на панели «" + pce.Node + "» уже есть клиент с таким именем — удалите его там вручную или выберите другое имя"}, nil
	case errors.Is(err, entity.ErrInvalidUserName):
		slog.Warn("handler user_create: invalid name", "name", req.Name)
		return &oas.UserCreateBadRequest{ErrMessage: msgInvalidName}, nil
	case errors.Is(err, entity.ErrNoConnectionSelected):
		slog.Warn("handler user_create: no connection selected", "name", req.Name)
		return &oas.UserCreateBadRequest{ErrMessage: msgNoConnection}, nil
	case errors.Is(err, entity.ErrInboundNotFound):
		slog.Warn("handler user_create: inbound not found", "name", req.Name)
		return &oas.UserCreateBadRequest{ErrMessage: msgInboundNotFound}, nil
	case errors.Is(err, entity.ErrNodeNotFound):
		slog.Warn("handler user_create: node not found", "name", req.Name)
		return &oas.UserCreateBadRequest{ErrMessage: msgNodeNotFound}, nil
	default:
		slog.Error("handler user_create: create failed", "name", req.Name, "err", err)
		return nil, err
	}
}
