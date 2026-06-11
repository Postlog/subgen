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
// Exported so apitest can assert against them without duplicating the text.
const (
	MsgSaved = "Конфиг сохранён"

	MsgGroupNameEmpty   = "Укажите название proxy-группы"
	MsgGroupNameTaken   = "Proxy-группа с таким названием уже существует"
	MsgGroupUnknownType = "Неизвестный тип proxy-группы"
	MsgGroupNoMembers   = "Пустая proxy-группа"
	MsgGroupCycle       = "Proxy-группы образуют циклическую ссылку"
	MsgBadRef           = "Некорректная цель правила/элемента группы"
	MsgGroupRefRange    = "Ссылка на несуществующую группу"
	MsgUnknownRuleType  = "Неизвестный тип правила"
	MsgMatchNotLast     = "Правило MATCH должно быть последним"
	MsgRuleValueReq     = "У правила не указано значение"
	MsgBaseYAMLInvalid  = "YAML невалиден — проверьте синтаксис"
	MsgGeneratedKey     = "Уберите из YAML генерируемые разделы"

	MsgProviderNameEmpty   = "Укажите название rule-provider"
	MsgProviderBadBehavior = "Неизвестный behavior у rule-provider"
	MsgProviderBadFormat   = "Неизвестный format у rule-provider"
	MsgProviderURLEmpty    = "Укажите URL у rule-provider"
	MsgRuleSetUnknownProv  = "RULE-SET ссылается на несуществующего rule-provider"

	MsgProfileTitleEmpty      = "Укажите название профиля (Profile title)"
	MsgProfileFilenameEmpty   = "Укажите имя файла подписки"
	MsgProfileFilenameInvalid = "Имя файла не должно содержать / \\ или управляющие символы"
	MsgProfileIntervalInvalid = "Интервал обновления — положительное число часов"

	MsgUserConfigMissing = "У пользователя нет кастомного конфига"
	MsgProviderNameTaken = "Rule-provider с таким именем уже существует"
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

	if err == nil {
		err = mihomo.ValidateProfile(profile)
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
			return &oas.ConfigSaveBadRequest{ErrMessage: MsgUserConfigMissing}, nil
		}

		slog.Error("handler config_save: resolve config failed", "userId", req.UserId.Or(0), "err", err)

		return nil, err
	}

	if err := h.routing.SaveMihomoConfig(ctx, configID, rules, groups, provs, base, profile); err != nil {
		if errors.Is(err, entity.ErrRuleProviderNameTaken) {
			slog.Warn("handler config_save: rule-provider name taken", "configID", configID)
			return &oas.ConfigSaveBadRequest{ErrMessage: MsgProviderNameTaken}, nil
		}

		slog.Error("handler config_save: save failed", "configID", configID, "err", err)

		return nil, err
	}

	return &oas.MessageResponse{Message: MsgSaved}, nil
}

// validationMessage maps a mihomo decode/validate sentinel to its user-facing text,
// reporting whether err was such a (4xx) validation failure. A false return means the
// error is unexpected and should be treated as a 500.
func validationMessage(err error) (string, bool) {
	switch {
	case errors.Is(err, mihomo.ErrGroupNameEmpty):
		return MsgGroupNameEmpty, true
	case errors.Is(err, mihomo.ErrGroupNameTaken):
		return MsgGroupNameTaken, true
	case errors.Is(err, mihomo.ErrGroupUnknownType):
		return MsgGroupUnknownType, true
	case errors.Is(err, mihomo.ErrGroupNoMembers):
		return MsgGroupNoMembers, true
	case errors.Is(err, mihomo.ErrGroupCycle):
		return MsgGroupCycle, true
	case errors.Is(err, mihomo.ErrBadRef):
		return MsgBadRef, true
	case errors.Is(err, mihomo.ErrGroupRefRange):
		return MsgGroupRefRange, true
	case errors.Is(err, mihomo.ErrUnknownRuleType):
		return MsgUnknownRuleType, true
	case errors.Is(err, mihomo.ErrMatchNotLast):
		return MsgMatchNotLast, true
	case errors.Is(err, mihomo.ErrRuleValueRequired):
		return MsgRuleValueReq, true
	case errors.Is(err, mihomo.ErrBaseYAMLInvalid):
		return MsgBaseYAMLInvalid, true
	case errors.Is(err, mihomo.ErrGeneratedKeyPresent):
		return MsgGeneratedKey, true
	case errors.Is(err, mihomo.ErrProviderNameEmpty):
		return MsgProviderNameEmpty, true
	case errors.Is(err, mihomo.ErrProviderBadBehavior):
		return MsgProviderBadBehavior, true
	case errors.Is(err, mihomo.ErrProviderBadFormat):
		return MsgProviderBadFormat, true
	case errors.Is(err, mihomo.ErrProviderURLEmpty):
		return MsgProviderURLEmpty, true
	case errors.Is(err, mihomo.ErrRuleSetUnknownProvider):
		return MsgRuleSetUnknownProv, true
	case errors.Is(err, mihomo.ErrProfileTitleEmpty):
		return MsgProfileTitleEmpty, true
	case errors.Is(err, mihomo.ErrProfileFilenameEmpty):
		return MsgProfileFilenameEmpty, true
	case errors.Is(err, mihomo.ErrProfileFilenameInvalid):
		return MsgProfileFilenameInvalid, true
	case errors.Is(err, mihomo.ErrProfileUpdateIntervalInvalid):
		return MsgProfileIntervalInvalid, true
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
