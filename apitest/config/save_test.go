//go:build apitest

package config_test

import (
	"github.com/postlog/subgen/apitest/api"
	configSaveHandler "github.com/postlog/subgen/internal/handlers/config_save"
)

// Corner cases considered for POST /admin/api/config/mihomo/save. Validation is ordered
// (base YAML → groups → rules → providers → RULE-SET refs) and short-circuits, so each
// rejected case is built to PASS every earlier check and trip exactly the one under
// test, and asserts the EXACT Russian message:
//   - happy.round_trip       — a small valid config saves and reads back identically.
//   - err.match_not_last      — a MATCH followed by another rule → "MATCH должно быть последним".
//   - err.rule_value_required — a non-MATCH rule with no value → "не указано значение".
//   - err.group_no_members    — a proxy-group with no members → "Пустая proxy-группа".
//   - err.group_name_taken    — two groups with the same name → "...уже существует".
//   - err.group_cycle         — A→B and B→A by index → "циклическую ссылку".
//   - err.group_ref_range     — a rule target group index out of range → "несуществующую группу".
//   - err.provider_nameless   — a provider with an empty name → "Укажите название rule-provider".
//   - err.provider_dup_name   — two valid providers sharing a name → "...уже существует" (DB PK).
//   - err.ruleset_unknown     — a RULE-SET naming an undefined provider → "RULE-SET ссылается...".
//   - err.generated_key       — base YAML carrying `proxies:` → "Уберите ... генерируемые разделы".
//   - err.base_yaml_invalid   — unparseable base YAML → "YAML невалиден".
//   - err.malformed_json      — a non-JSON body → MsgBadRequest.

// TestSaveRoundTrip covers the happy path: a fresh store accepts a valid config and
// reads it back intact.
func (s *ConfigSuite) TestSaveRoundTrip() {
	want := api.Config{
		BaseYAML: "mode: rule\n",
		Rules: []api.ConfigRule{
			{Type: "MATCH", Target: api.ConfigRef{Kind: "direct"}},
		},
	}

	res, err := s.api.SaveConfig(want)
	s.Require().NoError(err)
	s.Require().True(res.OK, "valid config must save: %s", res.Message())
	s.Equal(configSaveHandler.MsgSaved, res.Msg)

	got, err := s.api.ReadConfig()
	s.Require().NoError(err)
	s.Equal("mode: rule\n", got.BaseYAML)
	s.Require().Len(got.Rules, 1)
	s.Equal("MATCH", got.Rules[0].Type)
	s.Equal("direct", got.Rules[0].Target.Kind)

	// Reset the store back to empty so the populated config doesn't leak into the
	// read/empty test ordering (suite shares one store).
	s.T().Cleanup(func() { _, _ = s.api.SaveConfig(api.Config{}) })
}

// TestSaveValidation covers every rejected save with its exact friendly message.
func (s *ConfigSuite) TestSaveValidation() {
	s.Run("match_not_last", func() {
		s.saveRejected(api.Config{Rules: []api.ConfigRule{
			{Type: "MATCH", Target: api.ConfigRef{Kind: "direct"}},
			{Type: "DOMAIN-SUFFIX", Value: "example.com", Target: api.ConfigRef{Kind: "reject"}},
		}}, configSaveHandler.MsgMatchNotLast)
	})

	s.Run("rule_value_required", func() {
		// A non-MATCH rule with an empty value.
		s.saveRejected(api.Config{Rules: []api.ConfigRule{
			{Type: "DOMAIN-SUFFIX", Value: "", Target: api.ConfigRef{Kind: "direct"}},
		}}, configSaveHandler.MsgRuleValueReq)
	})

	s.Run("group_no_members", func() {
		s.saveRejected(api.Config{Groups: []api.ConfigGroup{
			{Name: "G", Type: "select"}, // no members
		}}, configSaveHandler.MsgGroupNoMembers)
	})

	s.Run("group_name_taken", func() {
		s.saveRejected(api.Config{Groups: []api.ConfigGroup{
			{Name: "DUP", Type: "select", Members: []api.ConfigRef{{Kind: "direct"}}},
			{Name: "DUP", Type: "select", Members: []api.ConfigRef{{Kind: "reject"}}},
		}}, configSaveHandler.MsgGroupNameTaken)
	})

	s.Run("group_cycle", func() {
		// Group 0 → group 1, group 1 → group 0 (by index).
		s.saveRejected(api.Config{Groups: []api.ConfigGroup{
			{Name: "A", Type: "select", Members: []api.ConfigRef{groupRef(1)}},
			{Name: "B", Type: "select", Members: []api.ConfigRef{groupRef(0)}},
		}}, configSaveHandler.MsgGroupCycle)
	})

	s.Run("group_ref_range", func() {
		// A rule whose target points at group index 5, but there are no groups.
		s.saveRejected(api.Config{Rules: []api.ConfigRule{
			{Type: "DOMAIN-SUFFIX", Value: "x.com", Target: groupRef(5)},
		}}, configSaveHandler.MsgGroupRefRange)
	})

	s.Run("provider_nameless", func() {
		s.saveRejected(api.Config{Providers: []api.ConfigProvider{
			{Name: "", Behavior: "domain", Format: "yaml", URL: "https://example.com/x.yaml"},
		}}, configSaveHandler.MsgProviderNameEmpty)
	})

	s.Run("provider_dup_name", func() {
		// Both fully valid → pass mihomo validation; the DB PK rejects the duplicate.
		s.saveRejected(api.Config{Providers: []api.ConfigProvider{
			{Name: "same", Behavior: "domain", Format: "yaml", URL: "https://example.com/a.yaml"},
			{Name: "same", Behavior: "domain", Format: "yaml", URL: "https://example.com/b.yaml"},
		}}, configSaveHandler.MsgProviderNameTaken)
	})

	s.Run("ruleset_unknown_provider", func() {
		// A RULE-SET naming a provider that isn't defined.
		s.saveRejected(api.Config{Rules: []api.ConfigRule{
			{Type: "RULE-SET", Value: "nope", Target: api.ConfigRef{Kind: "direct"}},
		}}, configSaveHandler.MsgRuleSetUnknownProv)
	})

	s.Run("generated_key_in_base", func() {
		s.saveRejected(api.Config{BaseYAML: "proxies:\n  - {name: x}\n"}, configSaveHandler.MsgGeneratedKey)
	})

	s.Run("base_yaml_invalid", func() {
		// Unparseable YAML (a bare unterminated flow mapping).
		s.saveRejected(api.Config{BaseYAML: "mode: [rule\n"}, configSaveHandler.MsgBaseYAMLInvalid)
	})

	s.Run("malformed_json", func() {
		res, err := s.api.SaveConfigRaw([]byte("{not json"))
		s.Require().NoError(err)
		s.False(res.OK)
		s.Equal(api.MsgBadRequest, res.Err)
	})
}

// groupRef builds a wire group reference at the given array index.
func groupRef(idx int) api.ConfigRef { return api.ConfigRef{Kind: "group", GroupIdx: &idx} }
