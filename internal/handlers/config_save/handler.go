// Package config_save implements the configSave operation
// (POST /admin/api/config/mihomo/save) — replace the base (or a user's custom) mihomo
// config.
package config_save

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/mihomo"
	"github.com/postlog/subgen/internal/oas"
)

// User-facing messages for the domain errors this handler can surface: the mihomo
// decode/validate sentinels (4xx invalid config), plus the two store-level conflicts.
const (
	msgGroupNameEmpty   = "Укажите название proxy-группы"
	msgGroupNameTaken   = "Proxy-группа с таким названием уже существует"
	msgGroupUnknownType = "Неизвестный тип proxy-группы"
	msgGroupNoMembers   = "Пустая proxy-группа"
	msgGroupCycle       = "Proxy-группы образуют циклическую ссылку"
	msgBadRef           = "Некорректная цель правила/элемента группы"
	msgGroupRefRange    = "Ссылка на несуществующую группу"
	msgUnknownRuleType  = "Неизвестный тип правила"
	msgMatchNotLast     = "Правило MATCH должно быть последним"
	msgRuleValueReq     = "У правила не указано значение"
	msgBaseYAMLInvalid  = "YAML невалиден — проверьте синтаксис"
	msgGeneratedKey     = "Уберите из YAML генерируемые разделы"

	msgProviderNameEmpty   = "Укажите название rule-provider"
	msgProviderBadBehavior = "Неизвестный behavior у rule-provider"
	msgProviderBadFormat   = "Неизвестный format у rule-provider"
	msgProviderURLEmpty    = "Укажите URL у rule-provider"
	msgRuleSetUnknownProv  = "RULE-SET ссылается на несуществующего rule-provider"

	msgUserConfigMissing = "У пользователя нет кастомного конфига"
	msgProviderNameTaken = "Rule-provider с таким именем уже существует"
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
// run through the mihomo decode + validation (the single source of those rules); a
// validation failure (or a store-level conflict) is a 400, an unexpected store/marshal
// failure is a 500.
func (h *Handler) ConfigSave(ctx context.Context, req *oas.ConfigSaveReq) (oas.ConfigSaveRes, error) {
	raw, err := json.Marshal(req)
	if err != nil {
		slog.Error("handler config_save: marshal request failed", "err", err)
		return nil, err
	}

	rules, groups, provs, base, profile, err := mihomo.DecodeConfig(raw)
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
		if msg, ok := validationMessage(err); ok {
			slog.Warn("handler config_save: invalid config", "err", err)
			return &oas.ConfigSaveBadRequest{ErrMessage: msg}, nil
		}

		slog.Error("handler config_save: decode/validate failed", "err", err)

		return nil, err
	}

	// Resolve the save scope only after validation, so invalid input never lazily
	// creates the base config row.
	configID, err := h.resolveConfigID(ctx, req.UserId.Or(0))
	if err != nil {
		if errors.Is(err, entity.ErrUserConfigNotFound) {
			slog.Warn("handler config_save: user has no custom config", "userId", req.UserId.Or(0))
			return &oas.ConfigSaveBadRequest{ErrMessage: msgUserConfigMissing}, nil
		}

		slog.Error("handler config_save: resolve config failed", "userId", req.UserId.Or(0), "err", err)

		return nil, err
	}

	if err := h.routing.SaveMihomoConfig(ctx, configID, rules, groups, provs, base, profile); err != nil {
		if errors.Is(err, entity.ErrRuleProviderNameTaken) {
			slog.Warn("handler config_save: rule-provider name taken", "configID", configID)
			return &oas.ConfigSaveBadRequest{ErrMessage: msgProviderNameTaken}, nil
		}

		slog.Error("handler config_save: save failed", "configID", configID, "err", err)

		return nil, err
	}

	return &oas.MessageResponse{Message: "Конфиг сохранён"}, nil
}

// validationMessage maps a mihomo decode/validate sentinel to its user-facing text,
// reporting whether err was such a (4xx) validation failure. A false return means the
// error is unexpected and should be treated as a 500.
func validationMessage(err error) (string, bool) {
	switch {
	case errors.Is(err, mihomo.ErrGroupNameEmpty):
		return msgGroupNameEmpty, true
	case errors.Is(err, mihomo.ErrGroupNameTaken):
		return msgGroupNameTaken, true
	case errors.Is(err, mihomo.ErrGroupUnknownType):
		return msgGroupUnknownType, true
	case errors.Is(err, mihomo.ErrGroupNoMembers):
		return msgGroupNoMembers, true
	case errors.Is(err, mihomo.ErrGroupCycle):
		return msgGroupCycle, true
	case errors.Is(err, mihomo.ErrBadRef):
		return msgBadRef, true
	case errors.Is(err, mihomo.ErrGroupRefRange):
		return msgGroupRefRange, true
	case errors.Is(err, mihomo.ErrUnknownRuleType):
		return msgUnknownRuleType, true
	case errors.Is(err, mihomo.ErrMatchNotLast):
		return msgMatchNotLast, true
	case errors.Is(err, mihomo.ErrRuleValueRequired):
		return msgRuleValueReq, true
	case errors.Is(err, mihomo.ErrBaseYAMLInvalid):
		return msgBaseYAMLInvalid, true
	case errors.Is(err, mihomo.ErrGeneratedKeyPresent):
		return msgGeneratedKey, true
	case errors.Is(err, mihomo.ErrProviderNameEmpty):
		return msgProviderNameEmpty, true
	case errors.Is(err, mihomo.ErrProviderBadBehavior):
		return msgProviderBadBehavior, true
	case errors.Is(err, mihomo.ErrProviderBadFormat):
		return msgProviderBadFormat, true
	case errors.Is(err, mihomo.ErrProviderURLEmpty):
		return msgProviderURLEmpty, true
	case errors.Is(err, mihomo.ErrRuleSetUnknownProvider):
		return msgRuleSetUnknownProv, true
	default:
		return "", false
	}
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
