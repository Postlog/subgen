// Package config_schema handles GET /admin/api/config/mihomo/schema — the static
// "schema" the admin SPA renders its config UI from. It declares, per section, every
// fact the frontend needs so it hardcodes nothing: the fixed actions (built-in
// policies), the rule-provider options, and — per proxy-group type / rule type — its
// options plus which reference categories its members / target may point at
// (`items` / `destinations`). Built entirely from the mihomo catalogs (the single
// source).
package config_schema

import (
	"net/http"
	"sort"
	"strings"

	"github.com/postlog/subgen/internal/handlers/web"
	"github.com/postlog/subgen/internal/mihomo"
)

// Handler serves the (static) config schema.
type Handler struct {
	schema map[string]any
}

// New builds the handler, precomputing the schema once.
func New() *Handler { return &Handler{schema: build()} }

func (h *Handler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	web.JSON(w, h.schema)
}

// action is a fixed picker option (a built-in policy).
type action struct {
	Kind  string `json:"kind"`
	Label string `json:"label"`
}

// ruleType is one rule type with its options and the categories its target may point
// at (destinations).
type ruleType struct {
	Type              string   `json:"type"`
	TakesProvider     bool     `json:"takesProvider"`
	SupportsNoResolve bool     `json:"supportsNoResolve"`
	IsMatch           bool     `json:"isMatch"`
	Destinations      []string `json:"destinations"`
}

// groupType is one proxy-group type with its options and the categories its members
// may point at (items).
type groupType struct {
	Type            string   `json:"type"`
	UsesHealthCheck bool     `json:"usesHealthCheck"`
	UsesTolerance   bool     `json:"usesTolerance"`
	Items           []string `json:"items"`
}

func build() map[string]any {
	categories := categoryKeys() // the same allowed set for items and destinations

	// actions = built-in policies (label = the mihomo policy name).
	actions := make([]action, 0, len(mihomo.BuiltinPolicyKinds()))
	for _, k := range mihomo.BuiltinPolicyKinds() {
		actions = append(actions, action{Kind: string(k), Label: strings.ToUpper(string(k))})
	}

	rules := make([]ruleType, 0, len(mihomo.RuleTypeCatalog()))
	for t, o := range mihomo.RuleTypeCatalog() {
		rules = append(rules, ruleType{
			Type: t.String(), TakesProvider: o.TakesProvider, SupportsNoResolve: o.SupportsNoResolve,
			IsMatch: t.IsMatch(), Destinations: categories,
		})
	}

	sort.Slice(rules, func(i, j int) bool { return rules[i].Type < rules[j].Type })

	groups := make([]groupType, 0, len(mihomo.ProxyGroupTypeCatalog()))
	for g, o := range mihomo.ProxyGroupTypeCatalog() {
		groups = append(groups, groupType{
			Type: g.String(), UsesHealthCheck: o.UsesHealthCheck, UsesTolerance: o.UsesTolerance,
			Items: categories,
		})
	}

	sort.Slice(groups, func(i, j int) bool { return groups[i].Type < groups[j].Type })

	return map[string]any{
		"actions": actions,
		"ruleProvider": map[string]any{
			"behaviors": mihomo.RuleProviderBehaviors(),
			"formats":   mihomo.RuleProviderFormats(),
		},
		"proxyGroup":    map[string]any{"types": groups},
		"rules":         map[string]any{"types": rules},
		"generatedKeys": mihomo.GeneratedKeys(),
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
