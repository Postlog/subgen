// Package config_save handles POST /admin/config/save.
package config_save

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/postlog/subgen/internal/handlers/web"
	"github.com/postlog/subgen/internal/mihomo"
)

const msgSaved = "Сохранено"

// Handler validates and persists the mihomo config from the config form. The saved
// config is read live on the next /sub request, so there is nothing to reload.
type Handler struct {
	routing mihomoSaver
}

// New builds the handler.
func New(routing mihomoSaver) *Handler {
	return &Handler{routing: routing}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var raw json.RawMessage
	if err := web.DecodeJSON(r, &raw); err != nil {
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

	if err == nil {
		err = h.routing.SaveMihomoConfig(r.Context(), rules, groups, provs, base)
	}
	// The fetch path keeps the live SPA state (the group/rule rows) on error, so no
	// edits are lost — just report the outcome.
	if err != nil {
		slog.Warn("handler config_save: save failed", "err", err)
	}

	web.JSONResult(w, msgSaved, err)
}
