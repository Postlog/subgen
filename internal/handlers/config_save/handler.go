// Package config_save implements the configSave operation
// (POST /admin/api/config/mihomo/save) — replace the base (or a user's custom) mihomo
// config.
package config_save

import (
	"context"
	"encoding/json"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/handlers/web"
	"github.com/postlog/subgen/internal/mihomo"
	"github.com/postlog/subgen/internal/oas"
)

// Handler saves a mihomo config (base or a user's custom).
type Handler struct {
	configs configResolver
	routing mihomoSaver
}

// New builds the handler.
func New(configs configResolver, routing mihomoSaver) *Handler {
	return &Handler{configs: configs, routing: routing}
}

// ConfigSave implements oas.Handler. The config part of the request is re-encoded and
// run through the mihomo decode + validation (the single source of those rules); any
// failure is a 400.
func (h *Handler) ConfigSave(ctx context.Context, req *oas.ConfigSaveReq) (oas.ConfigSaveRes, error) {
	raw, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	rules, groups, provs, base, err := mihomo.DecodeConfig(raw)
	if err == nil {
		err = mihomo.ValidateBaseYAML(base)
	}

	if err == nil {
		err = mihomo.ValidateProxyGroups(groups)
	}

	if err == nil {
		err = mihomo.ValidateRoutingRules(rules, len(groups))
	}

	if err == nil {
		err = mihomo.ValidateRuleProviders(provs)
	}

	if err == nil {
		err = mihomo.ValidateRuleProviderRefs(rules, provs)
	}

	if err != nil {
		return &oas.ConfigSaveBadRequest{ErrMessage: web.UserMessage(err)}, nil
	}

	// Resolve the save scope only after validation, so invalid input never lazily
	// creates the base config row.
	configID, err := h.resolveConfigID(ctx, req.UserId.Or(0))
	if err != nil {
		return &oas.ConfigSaveBadRequest{ErrMessage: web.UserMessage(err)}, nil
	}

	if err := h.routing.SaveMihomoConfig(ctx, configID, rules, groups, provs, base); err != nil {
		return &oas.ConfigSaveBadRequest{ErrMessage: web.UserMessage(err)}, nil
	}

	return &oas.MessageResponse{Message: "Конфиг сохранён"}, nil
}

// resolveConfigID maps the save scope to a config id: userID 0 ensures/returns the
// base; otherwise the user's existing custom config (ErrUserConfigNotFound if none).
func (h *Handler) resolveConfigID(ctx context.Context, userID int64) (int64, error) {
	if userID == 0 {
		return h.configs.EnsureBaseConfigID(ctx, entity.ConfigKindMihomo)
	}

	id, ok, err := h.configs.UserConfigID(ctx, userID, entity.ConfigKindMihomo)
	if err != nil {
		return 0, err
	}

	if !ok {
		return 0, entity.ErrUserConfigNotFound
	}

	return id, nil
}
