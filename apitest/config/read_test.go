//go:build apitest

package config_test

import (
	"sort"

	"github.com/postlog/subgen/apitest/api"
)

// Corner cases considered for GET /admin/api/config/mihomo:
//   - empty_store — a fresh store returns no rules/groups/providers and an empty base.
//   (The populated read is covered by the round-trip in save_test.go.)
//
// And for GET /admin/api/config/mihomo/schema:
//   - sections_present — actions / ruleProvider / proxyGroup / rules / generatedKeys.
//   - rule_types_sorted / group_types_sorted — the type option lists are name-sorted.
//   - provider_options — behaviors + formats match the mihomo catalogs.
//   - generated_keys   — the injected top-level keys are advertised.

// TestReadEmpty covers the fresh-store read (no seed).
func (s *ConfigSuite) TestReadEmpty() {
	cfg, err := s.api.ReadConfig()
	s.Require().NoError(err)
	s.Empty(cfg.Rules, "fresh store has no rules")
	s.Empty(cfg.Groups, "fresh store has no proxy-groups")
	s.Empty(cfg.Providers, "fresh store has no rule-providers")
	s.Empty(cfg.BaseYAML, "fresh store has no base YAML")
}

// TestSchema covers the static schema endpoint: the sections the SPA renders the config
// UI from (so the frontend hardcodes nothing), and that the type lists are sorted.
func (s *ConfigSuite) TestSchema() {
	schema, err := s.api.Schema()
	s.Require().NoError(err)

	s.Run("sections_present", func() {
		for _, key := range []string{"actions", "ruleProvider", "proxyGroup", "rules", "generatedKeys"} {
			s.Contains(schema, key, "schema must advertise %q", key)
		}
	})

	s.Run("rule_types_sorted", func() {
		types := typeStrings(schema, "rules")
		s.Require().NotEmpty(types, "schema must list rule types")
		s.True(sort.StringsAreSorted(types), "rule types must be name-sorted: %v", types)
		s.Contains(types, "MATCH")
		s.Contains(types, "RULE-SET")
		s.Contains(types, "AND") // logical rules are offered
	})

	s.Run("group_types_sorted", func() {
		types := typeStrings(schema, "proxyGroup")
		s.Require().NotEmpty(types, "schema must list proxy-group types")
		s.True(sort.StringsAreSorted(types), "group types must be name-sorted: %v", types)
		s.Contains(types, "select")
	})

	s.Run("provider_options", func() {
		rp, ok := schema["ruleProvider"].(map[string]any)
		s.Require().True(ok, "ruleProvider must be an object")
		s.ElementsMatch([]any{"domain", "ipcidr", "classical"}, rp["behaviors"])
		s.ElementsMatch([]any{"mrs", "yaml", "text"}, rp["formats"])
	})

	s.Run("generated_keys", func() {
		s.ElementsMatch([]any{"proxies", "proxy-groups", "rules", "rule-providers", "sub-rules"}, schema["generatedKeys"])
	})
}

// typeStrings extracts the ordered "type" field of each entry under
// schema[section]["types"] (rules or proxyGroup). It tolerates the generic decode by
// reading through map[string]any.
func typeStrings(schema api.Schema, section string) []string {
	sec, ok := schema[section].(map[string]any)
	if !ok {
		return nil
	}

	list, ok := sec["types"].([]any)
	if !ok {
		return nil
	}

	out := make([]string, 0, len(list))
	for _, e := range list {
		if m, ok := e.(map[string]any); ok {
			if t, ok := m["type"].(string); ok {
				out = append(out, t)
			}
		}
	}

	return out
}
