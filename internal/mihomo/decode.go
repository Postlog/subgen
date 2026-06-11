package mihomo

import (
	"encoding/json"
	"strings"
)

// JSON request DTOs for the mihomo-config save (anti-corruption boundary: the draft
// types stay free of json tags). A "group" ref carries the referenced group's ARRAY
// INDEX (groupIdx); a RULE-SET rule carries the provider's array index (providerIdx) —
// the persisted ids are assigned by SaveMihomoConfig, so the caller can't know them yet.
type policyRefDTO struct {
	Kind      string `json:"kind"`
	InboundID *int64 `json:"inboundId"`
	GroupIdx  *int   `json:"groupIdx"`
}

type ruleDTO struct {
	Type        string       `json:"type"`
	Value       *string      `json:"value"`
	ProviderIdx *int         `json:"providerIdx"`
	NoResolve   bool         `json:"noResolve"`
	Target      policyRefDTO `json:"target"`
}

type groupDTO struct {
	Name      string         `json:"name"`
	Type      string         `json:"type"`
	URL       string         `json:"url"`
	Interval  *int           `json:"interval"`
	Tolerance *int           `json:"tolerance"`
	Lazy      *bool          `json:"lazy"`
	Members   []policyRefDTO `json:"members"`
}

type providerDTO struct {
	Name           string `json:"name"`
	Behavior       string `json:"behavior"`
	Format         string `json:"format"`
	URL            string `json:"url"`
	Interval       int    `json:"interval"`
	Mirror         bool   `json:"mirror"`
	MirrorInterval int    `json:"mirrorInterval"`
}

type configDTO struct {
	BaseYAML              string        `json:"baseYAML"`
	Groups                []groupDTO    `json:"groups"`
	Rules                 []ruleDTO     `json:"rules"`
	Providers             []providerDTO `json:"providers"`
	ProfileTitle          string        `json:"profileTitle"`
	Filename              string        `json:"filename"`
	ProfileUpdateInterval int           `json:"profileUpdateInterval"`
}

// DecodeConfig unmarshals the raw mihomo-config JSON (the admin form payload) into a
// ConfigDraft: routing rules, proxy-groups, rule-providers, base YAML and profile
// knobs, with group/provider references carried as array indices (the save-time
// convention). It only decodes/maps — the handler reads the body and reports errors.
// The profile fields are returned verbatim (trimmed); defaults are substituted by the
// consumer, not here.
func DecodeConfig(raw json.RawMessage) (ConfigDraft, error) {
	var dto configDTO
	if err := json.Unmarshal(raw, &dto); err != nil {
		return ConfigDraft{}, err
	}

	groups := make([]GroupDraft, 0, len(dto.Groups))
	for _, g := range dto.Groups {
		members := make([]RefDraft, 0, len(g.Members))
		for _, m := range g.Members {
			members = append(members, refFromDTO(m))
		}

		groups = append(groups, GroupDraft{
			Name:      strings.TrimSpace(g.Name),
			Type:      ProxyGroupType(strings.TrimSpace(g.Type)),
			URL:       strings.TrimSpace(g.URL),
			Interval:  g.Interval,
			Tolerance: g.Tolerance,
			Lazy:      g.Lazy,
			Members:   members,
		})
	}

	rules := make([]RuleDraft, 0, len(dto.Rules))
	for _, ru := range dto.Rules {
		rules = append(rules, RuleDraft{
			Type:        RuleType(strings.TrimSpace(ru.Type)),
			Value:       trimPtr(ru.Value),
			ProviderIdx: ru.ProviderIdx,
			NoResolve:   ru.NoResolve,
			Target:      refFromDTO(ru.Target),
		})
	}

	// Keep every provider (empty name included) so ValidateRuleProviders can reject a
	// nameless one — silently dropping it would let a half-filled row "save" as a no-op.
	provs := make([]RuleProvider, 0, len(dto.Providers))
	for _, p := range dto.Providers {
		provs = append(provs, RuleProvider{
			Name: strings.TrimSpace(p.Name), Behavior: p.Behavior, Format: p.Format,
			URL: strings.TrimSpace(p.URL), Interval: p.Interval,
			Mirror: p.Mirror, MirrorInterval: p.MirrorInterval,
		})
	}

	return ConfigDraft{
		Groups:    groups,
		Rules:     rules,
		Providers: provs,
		BaseYAML:  dto.BaseYAML,
		Profile: Profile{
			Title:          strings.TrimSpace(dto.ProfileTitle),
			Filename:       strings.TrimSpace(dto.Filename),
			UpdateInterval: dto.ProfileUpdateInterval,
		},
	}, nil
}

// trimPtr trims a wire string pointer; an absent or whitespace-only value becomes nil
// (a value-taking rule with no value then trips ErrRuleValueRequired in validation).
func trimPtr(s *string) *string {
	if s == nil {
		return nil
	}

	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}

	return &v
}

// refFromDTO maps a JSON policy ref to a RefDraft. A group ref carries the array index
// (the save convention); an inbound ref carries the real inbound id.
func refFromDTO(d policyRefDTO) RefDraft {
	ref := RefDraft{Kind: PolicyKind(strings.TrimSpace(d.Kind))}

	switch ref.Kind {
	case PolicyInbound:
		ref.InboundID = d.InboundID
	case PolicyGroup:
		ref.GroupIdx = d.GroupIdx
	}

	return ref
}
