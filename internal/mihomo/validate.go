package mihomo

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ValidateBaseYAML checks the operator's base YAML parses and carries no generated
// section (subgen injects proxies/proxy-groups/rules/rule-providers itself).
func ValidateBaseYAML(base string) error {
	var m map[string]any
	if err := yaml.Unmarshal([]byte(base), &m); err != nil {
		return fmt.Errorf("%w: %v", ErrBaseYAMLInvalid, err)
	}

	for _, k := range GeneratedKeys() {
		if _, ok := m[k]; ok {
			return ErrGeneratedKeyPresent
		}
	}

	return nil
}

// ValidateProxyGroups validates each group's intra-group invariant (GroupDraft.Valid:
// name, type, members, field applicability), then the cross-group checks: name
// uniqueness, every member ref well-formed and in range, and an acyclic group→group
// graph. Group refs use array indices.
func ValidateProxyGroups(groups []GroupDraft) error {
	seen := map[string]bool{}

	for _, g := range groups {
		if err := g.Valid(); err != nil {
			return err
		}

		if seen[g.Name] {
			return ErrGroupNameTaken
		}

		seen[g.Name] = true

		for _, m := range g.Members {
			if err := validateRef(m, len(groups)); err != nil {
				return err
			}
		}
	}

	if cyclicGroups(groups) {
		return ErrGroupCycle
	}

	return nil
}

// ValidateRoutingRules validates each rule's per-type invariant (RuleDraft.Valid: value/
// provider/no-resolve by type), then the cross-rule checks: MATCH (if present) is last,
// a RULE-SET's provider index is in range, and the target ref is well-formed/in range.
func ValidateRoutingRules(rules []RuleDraft, numGroups, numProviders int) error {
	for i, rule := range rules {
		if err := rule.Valid(); err != nil {
			return err
		}

		if rule.Type.IsMatch() && i != len(rules)-1 {
			return ErrMatchNotLast
		}

		// Valid() ensured a RULE-SET carries a provider index; check it is in range.
		if rule.Type.TakesProvider() && (*rule.ProviderIdx < 0 || *rule.ProviderIdx >= numProviders) {
			return ErrProviderRefRange
		}

		if err := validateRef(rule.Target, numGroups); err != nil {
			return err
		}
	}

	return nil
}

// ValidateRuleProviders checks each provider has a non-empty name, a known behavior
// and format, and a URL. Name uniqueness is NOT checked here — it is enforced by the
// table's PK and translated to entity.ErrRuleProviderNameTaken in the repository.
func ValidateRuleProviders(provs []RuleProvider) error {
	behaviors := sliceSet(RuleProviderBehaviors())
	formats := sliceSet(RuleProviderFormats())

	for _, p := range provs {
		switch {
		case p.Name == "":
			return ErrProviderNameEmpty
		case !behaviors[p.Behavior]:
			return ErrProviderBadBehavior
		case !formats[p.Format]:
			return ErrProviderBadFormat
		case p.URL == "":
			return ErrProviderURLEmpty
		}
	}

	return nil
}

// ValidateProfile checks the subscription-profile knobs: a non-empty title, a non-empty
// filename that is safe to place in a Content-Disposition header (no path separators or
// control characters), and a positive update interval. The interval is the Clash
// Profile-Update-Interval hint, which clients read as a whole number of HOURS — so any
// value below 1 is meaningless. Fields arrive trimmed from DecodeConfig.
func ValidateProfile(p Profile) error {
	if p.Title == "" {
		return ErrProfileTitleEmpty
	}

	if p.Filename == "" {
		return ErrProfileFilenameEmpty
	}

	if strings.ContainsAny(p.Filename, `/\`) || strings.IndexFunc(p.Filename, isControl) >= 0 {
		return ErrProfileFilenameInvalid
	}

	if p.UpdateInterval < 1 {
		return ErrProfileUpdateIntervalInvalid
	}

	return nil
}

// isControl reports whether r is an ASCII control character (covers CR/LF/TAB and the
// rest), which must not appear in a Content-Disposition filename.
func isControl(r rune) bool { return r < 0x20 || r == 0x7f }

func sliceSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}

	return m
}

// validateRef checks a save-time ref's internal consistency and that a group ref's
// index is in range.
func validateRef(ref RefDraft, numGroups int) error {
	if err := ref.Valid(); err != nil {
		return err
	}

	if ref.Kind == PolicyGroup {
		if ref.GroupIdx == nil || *ref.GroupIdx < 0 || *ref.GroupIdx >= numGroups {
			return ErrGroupRefRange
		}
	}

	return nil
}

// cyclicGroups reports whether the group→group reference graph (members of kind
// group, by index) contains a cycle.
func cyclicGroups(groups []GroupDraft) bool {
	const (
		white = 0
		gray  = 1
		black = 2
	)

	color := make([]int, len(groups))

	var visit func(i int) bool

	visit = func(i int) bool {
		color[i] = gray

		for _, m := range groups[i].Members {
			if m.Kind != PolicyGroup || m.GroupIdx == nil {
				continue
			}

			j := *m.GroupIdx
			if j < 0 || j >= len(groups) {
				continue
			}

			if color[j] == gray || (color[j] == white && visit(j)) {
				return true
			}
		}

		color[i] = black

		return false
	}

	for i := range groups {
		if color[i] == white && visit(i) {
			return true
		}
	}

	return false
}
