// Package config_customs implements the configCustoms operation
// (GET /admin/api/config/mihomo/customs) — the users that have a custom mihomo config
// as {userId,name} pairs, plus the full id+name user list for the "+ custom config"
// picker (so the config tab never loads the paged /admin/api/users).
package config_customs

import (
	"context"
	"log/slog"
	"sort"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/oas"
)

// Handler serves the custom-config owners + the full user list.
type Handler struct {
	configs configLister
	users   userLister
}

// New builds the handler.
func New(configs configLister, users userLister) *Handler {
	return &Handler{configs: configs, users: users}
}

// ConfigCustoms implements oas.Handler.
func (h *Handler) ConfigCustoms(ctx context.Context) (oas.ConfigCustomsRes, error) {
	ids, err := h.configs.UserConfigUserIDs(ctx, entity.ConfigKindMihomo)
	if err != nil {
		slog.Error("handler config_customs: list custom-config owners failed", "err", err)
		return nil, err
	}

	users, err := h.users.ListNames(ctx)
	if err != nil {
		slog.Error("handler config_customs: list user names failed", "err", err)
		return nil, err
	}

	name := make(map[int64]string, len(users))
	all := make([]oas.ConfigCustomsOKUsersItem, 0, len(users))

	for i := range users {
		name[users[i].ID] = users[i].Name
		all = append(all, oas.ConfigCustomsOKUsersItem{ID: users[i].ID, Name: users[i].Name})
	}

	customs := make([]oas.ConfigCustomsOKCustomsItem, 0, len(ids))
	for _, id := range ids {
		customs = append(customs, oas.ConfigCustomsOKCustomsItem{UserId: id, Name: name[id]})
	}

	sort.Slice(customs, func(i, j int) bool { return customs[i].Name < customs[j].Name })

	return &oas.ConfigCustomsOK{Customs: customs, Users: all}, nil
}
