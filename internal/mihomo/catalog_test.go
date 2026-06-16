package mihomo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuleTypeCatalog(t *testing.T) {
	t.Parallel()

	cat := RuleTypeCatalog()
	require.NotEmpty(t, cat)

	// Every catalogued type is Valid and its accessors agree with the catalog.
	for typ, opts := range cat {
		assert.Truef(t, typ.Valid(), "%s not Valid()", typ)
		assert.Equalf(t, opts.TakesProvider, typ.TakesProvider(), "%s TakesProvider() != catalog", typ)
		assert.Equalf(t, opts.SupportsNoResolve, typ.SupportsNoResolve(), "%s SupportsNoResolve() != catalog", typ)
		assert.Equalf(t, opts.Logical, typ.IsLogical(), "%s IsLogical() != catalog", typ)

		// A logical type never carries a value-matcher's options.
		if opts.Logical {
			assert.Falsef(t, opts.TakesProvider, "%s logical must not take a provider", typ)
			assert.Falsef(t, opts.SupportsNoResolve, "%s logical must not support no-resolve", typ)
		}
	}

	// Spot-check the load-bearing facts the schema/UI relies on.
	assert.True(t, RuleRuleSet.TakesProvider(), "RULE-SET should take a provider")

	assert.True(t, RuleGeoIP.SupportsNoResolve(), "GEOIP should support no-resolve")
	assert.False(t, RuleDomain.SupportsNoResolve(), "DOMAIN should not support no-resolve")

	assert.True(t, RuleMatch.IsMatch(), "MATCH should be IsMatch")
	assert.False(t, RuleDomain.IsMatch(), "DOMAIN should not be IsMatch")

	// Logical types are IsLogical; a plain matcher is not.
	for _, lt := range []RuleType{RuleAnd, RuleOr, RuleNot} {
		assert.Truef(t, lt.IsLogical(), "%s should be IsLogical", lt)
	}

	assert.False(t, RuleDomain.IsLogical(), "DOMAIN should not be IsLogical")

	// The four wiki-parity matchers are catalogued and are plain matchers (no special opts).
	for _, mt := range []RuleType{RuleSrcIPASN, RuleSrcIPSuffix, RuleProcessNameWild, RuleProcessPathWild} {
		assert.Truef(t, mt.Valid(), "%s should be Valid", mt)
		assert.Falsef(t, mt.TakesProvider(), "%s should not take a provider", mt)
		assert.Falsef(t, mt.SupportsNoResolve(), "%s should not support no-resolve", mt)
		assert.Falsef(t, mt.IsLogical(), "%s should not be logical", mt)
	}
}

func TestProxyGroupTypeCatalog(t *testing.T) {
	t.Parallel()

	cat := ProxyGroupTypeCatalog()
	require.NotEmpty(t, cat)

	for typ, opts := range cat {
		assert.Truef(t, typ.Valid(), "%s not Valid()", typ)
		assert.Equalf(t, opts.UsesHealthCheck, typ.UsesHealthCheck(), "%s UsesHealthCheck() != catalog", typ)
	}

	assert.True(t, cat[GroupURLTest].UsesHealthCheck, "url-test should use health-check")
	assert.True(t, cat[GroupURLTest].UsesTolerance, "url-test should use tolerance")

	assert.False(t, cat[GroupSelect].UsesHealthCheck, "select should not use health-check")
	assert.False(t, cat[GroupSelect].UsesTolerance, "select should not use tolerance")
}

func TestBuiltinPolicyKinds(t *testing.T) {
	t.Parallel()

	kinds := BuiltinPolicyKinds()
	require.NotEmpty(t, kinds)

	for _, k := range kinds {
		assert.Truef(t, k.Valid(), "%s not Valid()", k)
		assert.NotEqualf(t, PolicyInbound, k, "%s is a reference kind, not a built-in", k)
		assert.NotEqualf(t, PolicyGroup, k, "%s is a reference kind, not a built-in", k)
	}
}
