// Package config_save handles POST /admin/config/save.
package config_save

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/postlog/subgen/internal/entity"
	"github.com/postlog/subgen/internal/handlers/web"
	"github.com/postlog/subgen/internal/mihomo"
)

const msgSaved = "Сохранено"

// Handler validates and persists a mihomo config from the config form. The save
// scope is the base config, or a user's custom config when userId is set. The saved
// config is read live on the next /sub request, so there is nothing to reload.
type Handler struct {
	configs configResolver
	routing mihomoSaver
}

// New builds the handler.
func New(configs configResolver, routing mihomoSaver) *Handler {
	return &Handler{configs: configs, routing: routing}
}

// saveForm extends the config payload (decoded separately by mihomo.DecodeConfig)
// with the optional save scope: userId>0 targets that user's custom config.
type saveForm struct {
	UserID int64 `json:"userId"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var raw json.RawMessage
	if err := web.DecodeJSON(r, &raw); err != nil {
		slog.Warn("handler config_save: decode failed", "err", err)
		web.WriteJSON(w, false, web.MsgBadRequest)

		return
	}

	var form saveForm
	if err := json.Unmarshal(raw, &form); err != nil {
		slog.Warn("handler config_save: decode failed", "err", err)
		web.WriteJSON(w, false, web.MsgBadRequest)

		return
	}

	rules, groups, provs, base, err := mihomo.DecodeConfig(raw)
	if err != nil {
		slog.Warn("handler config_save: decode failed", "err", err)
		web.WriteJSON(w, false, web.MsgBadRequest)

		return
	}

	err = mihomo.ValidateBaseYAML(base)
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

	// Resolve the save scope only after validation passes, so invalid input never
	// lazily creates the base config row.
	var configID int64
	if err == nil {
		configID, err = h.resolveConfigID(r.Context(), form.UserID)
	}

	if err == nil {
		err = h.routing.SaveMihomoConfig(r.Context(), configID, rules, groups, provs, base)
	}
	// The fetch path keeps the live SPA state (the group/rule rows) on error, so no
	// edits are lost — just report the outcome.
	if err != nil {
		slog.Warn("handler config_save: save failed", "err", err)
	}

	web.JSONResult(w, msgSaved, err)
}

// resolveConfigID maps the save scope to a config id: userId==0 → the base config
// (lazily created on first save); userId>0 → that user's custom config (must already
// exist — creation is a separate action). Returns entity.ErrUserConfigNotFound when
// the user has no custom config.
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
