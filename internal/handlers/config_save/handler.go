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
	MsgSaved = "Config saved"

	MsgGroupNameEmpty   = "Enter a proxy-group name"
	MsgGroupNameTaken   = "A proxy-group with this name already exists"
	MsgGroupUnknownType = "Unknown proxy-group type"
	MsgGroupNoMembers   = "Empty proxy-group"
	MsgGroupCycle       = "Proxy-groups form a cyclic reference"
	MsgGroupFieldNA     = "This field does not apply to this proxy-group type"
	MsgBadRef           = "Invalid rule/group-member target"
	MsgGroupRefRange    = "Reference to a non-existent group"
	MsgUnknownRuleType  = "Unknown rule type"
	MsgMatchNotLast     = "The MATCH rule must be last"
	MsgRuleValueReq     = "The rule has no value"
	MsgRulePayloadNA    = "This rule type does not accept a value"
	MsgNoResolveNA      = "no-resolve does not apply to this rule type"
	MsgChildrenNA       = "Nested rules are allowed only for logical rules (AND/OR/NOT)"
	MsgNotArity         = "NOT must contain exactly one nested rule"
	MsgLogicalArity     = "AND/OR must contain at least two nested rules"
	MsgMatchChild       = "MATCH cannot be used as a nested rule"
	MsgTargetRequired   = "The rule has no target"
	MsgChildTarget      = "A nested rule must not have a target"
	MsgBaseYAMLInvalid  = "Invalid YAML — check the syntax"
	MsgGeneratedKey     = "Remove the generated sections from the YAML"

	MsgProviderNameEmpty   = "Enter a rule-provider name"
	MsgProviderBadSource   = "Unknown rule-provider source"
	MsgProviderBadBehavior = "Unknown rule-provider behavior"
	MsgProviderBadFormat   = "Unknown rule-provider format"
	MsgProviderURLEmpty    = "Enter the rule-provider URL"
	MsgRuleSetUnknownProv  = "RULE-SET references a non-existent rule-provider"

	MsgProviderAuthoredURLSet        = "An authored rule-provider must not have a URL"
	MsgProviderAuthoredNeedsMatchers = "Add at least one rule to the authored rule-provider"
	MsgProviderMatcherUnsupported    = "MATCH and RULE-SET cannot be used in an authored list"

	MsgProfileTitleEmpty             = "Enter the profile title (Profile title)"
	MsgProfileFilenameEmpty          = "Enter the subscription filename"
	MsgProfileFilenameInvalid        = "The filename must not contain / \\ or control characters"
	MsgProfileIntervalInvalid        = "Update interval must be a positive number of hours"
	MsgProfileProxiesIntervalInvalid = "Nodes update interval must be a positive number of seconds"

	MsgUserConfigMissing = "The user has no custom config"
	MsgProviderNameTaken = "A rule-provider with this name already exists"
)

// Handler saves a mihomo config (base or a user's custom).
type Handler struct {
	configs configsRepo
	routing routingRepo
}

// New builds the handler.
func New(configs configsRepo, routing routingRepo) *Handler {
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

	draft, err := mihomo.DecodeConfig(raw)
	if err == nil {
		err = mihomo.ValidateBaseYAML(draft.BaseYAML)
	}

	if err == nil {
		err = mihomo.ValidateProxyGroups(draft.Groups)
	}

	if err == nil {
		err = mihomo.ValidateRoutingRules(draft.Rules, len(draft.Groups), len(draft.Providers))
	}

	if err == nil {
		err = mihomo.ValidateRuleProviders(draft.Providers)
	}

	if err == nil {
		err = mihomo.ValidateProfile(draft.Profile)
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

	if err := h.routing.SaveMihomoConfig(ctx, configID, draft); err != nil {
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
	case errors.Is(err, mihomo.ErrGroupFieldNotAllowed):
		return MsgGroupFieldNA, true
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
	case errors.Is(err, mihomo.ErrRulePayloadNotAllowed):
		return MsgRulePayloadNA, true
	case errors.Is(err, mihomo.ErrNoResolveUnsupported):
		return MsgNoResolveNA, true
	case errors.Is(err, mihomo.ErrChildrenNotAllowed):
		return MsgChildrenNA, true
	case errors.Is(err, mihomo.ErrNotArity):
		return MsgNotArity, true
	case errors.Is(err, mihomo.ErrLogicalArity):
		return MsgLogicalArity, true
	case errors.Is(err, mihomo.ErrMatchChild):
		return MsgMatchChild, true
	case errors.Is(err, mihomo.ErrTargetRequired):
		return MsgTargetRequired, true
	case errors.Is(err, mihomo.ErrChildTarget):
		return MsgChildTarget, true
	case errors.Is(err, mihomo.ErrBaseYAMLInvalid):
		return MsgBaseYAMLInvalid, true
	case errors.Is(err, mihomo.ErrGeneratedKeyPresent):
		return MsgGeneratedKey, true
	case errors.Is(err, mihomo.ErrProviderNameEmpty):
		return MsgProviderNameEmpty, true
	case errors.Is(err, mihomo.ErrProviderBadSource):
		return MsgProviderBadSource, true
	case errors.Is(err, mihomo.ErrProviderBadBehavior):
		return MsgProviderBadBehavior, true
	case errors.Is(err, mihomo.ErrProviderBadFormat):
		return MsgProviderBadFormat, true
	case errors.Is(err, mihomo.ErrProviderURLEmpty):
		return MsgProviderURLEmpty, true
	case errors.Is(err, mihomo.ErrProviderRefRange):
		return MsgRuleSetUnknownProv, true
	case errors.Is(err, mihomo.ErrProviderAuthoredURLSet):
		return MsgProviderAuthoredURLSet, true
	case errors.Is(err, mihomo.ErrProviderAuthoredNeedsMatchers):
		return MsgProviderAuthoredNeedsMatchers, true
	case errors.Is(err, mihomo.ErrProviderMatcherUnsupported):
		return MsgProviderMatcherUnsupported, true
	case errors.Is(err, mihomo.ErrProfileTitleEmpty):
		return MsgProfileTitleEmpty, true
	case errors.Is(err, mihomo.ErrProfileFilenameEmpty):
		return MsgProfileFilenameEmpty, true
	case errors.Is(err, mihomo.ErrProfileFilenameInvalid):
		return MsgProfileFilenameInvalid, true
	case errors.Is(err, mihomo.ErrProfileUpdateIntervalInvalid):
		return MsgProfileIntervalInvalid, true
	case errors.Is(err, mihomo.ErrProfileProxiesIntervalInvalid):
		return MsgProfileProxiesIntervalInvalid, true
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
