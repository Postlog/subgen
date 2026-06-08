package config_schema

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postlog/subgen/internal/oas"
)

func TestHandler_ConfigSchema(t *testing.T) {
	t.Parallel()

	res, err := New().ConfigSchema(context.Background())
	require.NoError(t, err)

	ok, isOK := res.(*oas.ConfigSchemaOK)
	require.True(t, isOK, "want *oas.ConfigSchemaOK, got %T", res)

	assert.NotEmpty(t, ok.Actions)
	assert.NotEmpty(t, ok.RuleProvider.Behaviors)
	assert.NotEmpty(t, ok.RuleProvider.Formats)
	assert.NotEmpty(t, ok.ProxyGroup.Types)
	assert.NotEmpty(t, ok.Rules.Types)
	assert.NotEmpty(t, ok.GeneratedKeys)
}
