package config_schema

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// New() is dependency-free (no contract.go), so the handler needs no mocks. The
// schema is static, so a single straight-line test exercises the one ServeHTTP path
// and asserts the catalog the SPA depends on.
func TestHandler_ServeHTTP(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()

	New().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/api/config/mihomo/schema", nil))

	require.Equal(t, http.StatusOK, rec.Code)

	var s struct {
		Actions []struct {
			Kind, Label string
		} `json:"actions"`
		RuleProvider struct {
			Behaviors []string `json:"behaviors"`
			Formats   []string `json:"formats"`
		} `json:"ruleProvider"`
		ProxyGroup struct {
			Types []struct {
				Type            string   `json:"type"`
				UsesHealthCheck bool     `json:"usesHealthCheck"`
				UsesTolerance   bool     `json:"usesTolerance"`
				Items           []string `json:"items"`
			} `json:"types"`
		} `json:"proxyGroup"`
		Rules struct {
			Types []struct {
				Type          string   `json:"type"`
				TakesProvider bool     `json:"takesProvider"`
				IsMatch       bool     `json:"isMatch"`
				Destinations  []string `json:"destinations"`
			} `json:"types"`
		} `json:"rules"`
		GeneratedKeys []string `json:"generatedKeys"`
	}

	require.NoError(t, json.NewDecoder(rec.Body).Decode(&s))

	// actions = built-in policies only; reference kinds (inbound/group) are not actions.
	assert.Truef(t, hasKind(s.Actions, "direct"), "actions must include built-ins: %+v", s.Actions)
	assert.Falsef(t, hasKind(s.Actions, "inbound"), "reference kinds must NOT be actions: %+v", s.Actions)
	assert.Falsef(t, hasKind(s.Actions, "group"), "reference kinds must NOT be actions: %+v", s.Actions)

	require.NotEmpty(t, s.Rules.Types, "rules catalog is empty")
	require.NotEmpty(t, s.ProxyGroup.Types, "proxyGroup catalog is empty")

	// Rule types sorted by name, each carrying destinations.
	names := make([]string, len(s.Rules.Types))
	for i, r := range s.Rules.Types {
		names[i] = r.Type
		assert.NotEmptyf(t, r.Destinations, "%s: no destinations", r.Type)
	}

	assert.Truef(t, sort.StringsAreSorted(names), "rule types not sorted by name: %v", names)

	for _, r := range s.Rules.Types {
		switch r.Type {
		case "RULE-SET":
			assert.True(t, r.TakesProvider, "RULE-SET should takeProvider")
		case "MATCH":
			assert.True(t, r.IsMatch, "MATCH should be isMatch")
		}
	}

	// Group types carry items; url-test uses health-check + tolerance.
	for _, g := range s.ProxyGroup.Types {
		assert.NotEmptyf(t, g.Items, "%s: no items", g.Type)

		if g.Type == "url-test" {
			assert.True(t, g.UsesHealthCheck, "url-test should use health-check")
			assert.True(t, g.UsesTolerance, "url-test should use tolerance")
		}
	}

	assert.NotEmpty(t, s.RuleProvider.Behaviors, "rule-provider behaviors missing")
	assert.NotEmpty(t, s.RuleProvider.Formats, "rule-provider formats missing")
	assert.NotEmpty(t, s.GeneratedKeys, "generatedKeys missing")
}

func hasKind(actions []struct{ Kind, Label string }, kind string) bool {
	for _, a := range actions {
		if a.Kind == kind {
			return true
		}
	}

	return false
}
