// Package config_schema implements the configSchema operation
// (GET /admin/api/config/mihomo/schema) — the static catalog the admin SPA renders its
// config UI from, built entirely from the mihomo catalogs (the single source).
package config_schema

import (
	"context"
	"sort"
	"strings"

	"github.com/postlog/subgen/internal/mihomo"
	"github.com/postlog/subgen/internal/oas"
)

// Handler serves the (static) config schema, precomputed once.
type Handler struct {
	schema oas.ConfigSchemaOK
}

// New builds the handler, precomputing the schema once.
func New() *Handler { return &Handler{schema: build()} }

// ConfigSchema implements oas.Handler.
func (h *Handler) ConfigSchema(_ context.Context) (oas.ConfigSchemaRes, error) {
	s := h.schema
	return &s, nil
}

func build() oas.ConfigSchemaOK {
	categories := categoryKeys() // the same allowed set for items and destinations

	actions := make([]oas.ConfigSchemaOKActionsItem, 0, len(mihomo.BuiltinPolicyKinds()))
	for _, k := range mihomo.BuiltinPolicyKinds() {
		actions = append(actions, oas.ConfigSchemaOKActionsItem{Kind: string(k), Label: strings.ToUpper(string(k))})
	}

	rules := make([]oas.ConfigSchemaOKRulesTypesItem, 0, len(mihomo.RuleTypeCatalog()))
	for t, o := range mihomo.RuleTypeCatalog() {
		rules = append(rules, oas.ConfigSchemaOKRulesTypesItem{
			Type: t.String(), TakesProvider: o.TakesProvider, SupportsNoResolve: o.SupportsNoResolve,
			IsMatch: t.IsMatch(), IsLogical: o.Logical, Destinations: categories,
		})
	}

	sort.Slice(rules, func(i, j int) bool { return rules[i].Type < rules[j].Type })

	groups := make([]oas.ConfigSchemaOKProxyGroupTypesItem, 0, len(mihomo.ProxyGroupTypeCatalog()))
	for g, o := range mihomo.ProxyGroupTypeCatalog() {
		groups = append(groups, oas.ConfigSchemaOKProxyGroupTypesItem{
			Type: g.String(), UsesHealthCheck: o.UsesHealthCheck, UsesTolerance: o.UsesTolerance,
			Items: categories,
		})
	}

	sort.Slice(groups, func(i, j int) bool { return groups[i].Type < groups[j].Type })

	return oas.ConfigSchemaOK{
		Actions:       actions,
		RuleProvider:  oas.ConfigSchemaOKRuleProvider{Sources: mihomo.RuleProviderSources(), Behaviors: mihomo.RuleProviderBehaviors(), Formats: mihomo.RuleProviderFormats()},
		ProxyGroup:    oas.ConfigSchemaOKProxyGroup{Types: groups},
		Rules:         oas.ConfigSchemaOKRules{Types: rules},
		GeneratedKeys: mihomo.GeneratedKeys(),
	}
}

// categoryKeys returns the allowed reference categories as wire strings.
func categoryKeys() []string {
	cats := mihomo.PolicyCategories()
	out := make([]string, len(cats))

	for i, c := range cats {
		out[i] = string(c)
	}

	return out
}
