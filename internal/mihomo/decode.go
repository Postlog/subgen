package mihomo

import (
	"encoding/json"
	"strings"
)

// JSON request DTOs for the mihomo-config save (anti-corruption boundary: the domain
// types stay free of json tags). A "group" PolicyRef carries the referenced group's
// ARRAY INDEX (groupIdx), not a persisted id — the convention SaveMihomoConfig wants.
type policyRefDTO struct {
	Kind      string `json:"kind"`
	InboundID *int64 `json:"inboundId"`
	GroupIdx  *int   `json:"groupIdx"`
}

type ruleDTO struct {
	Type      string       `json:"type"`
	Value     string       `json:"value"`
	NoResolve bool         `json:"noResolve"`
	Target    policyRefDTO `json:"target"`
}

type groupDTO struct {
	Name      string         `json:"name"`
	Type      string         `json:"type"`
	URL       string         `json:"url"`
	Interval  int            `json:"interval"`
	Tolerance int            `json:"tolerance"`
	Lazy      bool           `json:"lazy"`
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

// DecodeConfig unmarshals the raw mihomo-config JSON (the admin form payload) into the
// structured routing rules, proxy-groups, rule-providers, base YAML and profile knobs.
// It only decodes/maps — the handler reads the request body and reports errors. The
// profile fields are returned verbatim (trimmed); defaults are substituted by the
// consumer, not here.
func DecodeConfig(raw json.RawMessage) (rules []RoutingRule, groups []ProxyGroup, provs []RuleProvider, base string, profile Profile, err error) {
	var dto configDTO
	if err := json.Unmarshal(raw, &dto); err != nil {
		return nil, nil, nil, "", Profile{}, err
	}

	groups = make([]ProxyGroup, 0, len(dto.Groups))
	for _, g := range dto.Groups {
		members := make([]PolicyRef, 0, len(g.Members))
		for _, m := range g.Members {
			members = append(members, refFromDTO(m))
		}

		groups = append(groups, ProxyGroup{
			Name:      strings.TrimSpace(g.Name),
			Type:      ProxyGroupType(strings.TrimSpace(g.Type)),
			URL:       strings.TrimSpace(g.URL),
			Interval:  g.Interval,
			Tolerance: g.Tolerance,
			Lazy:      g.Lazy,
			Members:   members,
		})
	}

	rules = make([]RoutingRule, 0, len(dto.Rules))
	for _, ru := range dto.Rules {
		rules = append(rules, RoutingRule{
			Type:      RuleType(strings.TrimSpace(ru.Type)),
			Value:     strings.TrimSpace(ru.Value),
			NoResolve: ru.NoResolve,
			Target:    refFromDTO(ru.Target),
		})
	}

	// Keep every provider (empty name included) so ValidateRuleProviders can reject a
	// nameless one — silently dropping it would let a half-filled row "save" as a no-op.
	for _, p := range dto.Providers {
		provs = append(provs, RuleProvider{
			Name: strings.TrimSpace(p.Name), Behavior: p.Behavior, Format: p.Format,
			URL: strings.TrimSpace(p.URL), Interval: p.Interval,
			Mirror: p.Mirror, MirrorInterval: p.MirrorInterval,
		})
	}

	profile = Profile{
		Title:          strings.TrimSpace(dto.ProfileTitle),
		Filename:       strings.TrimSpace(dto.Filename),
		UpdateInterval: dto.ProfileUpdateInterval,
	}

	return rules, groups, provs, dto.BaseYAML, profile, nil
}

// refFromDTO maps a JSON policy ref to a PolicyRef. A group ref carries the array
// index (the save convention); an inbound ref carries the real inbound id.
func refFromDTO(d policyRefDTO) PolicyRef {
	ref := PolicyRef{Kind: PolicyKind(strings.TrimSpace(d.Kind))}

	switch ref.Kind {
	case PolicyInbound:
		ref.InboundID = d.InboundID
	case PolicyGroup:
		if d.GroupIdx != nil {
			idx := int64(*d.GroupIdx)
			ref.GroupID = &idx
		}
	}

	return ref
}
