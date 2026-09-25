package validation

import (
	"io"
	"strings"
	"testing"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateTarget_WarnsOnUnknownFields(t *testing.T) {
	t.Parallel()

	target, err := generate.GetTargetFromTargetString("go")
	require.NoError(t, err)

	fields, err := generate.GetLanguageConfigFields(target, false)
	require.NoError(t, err)
	require.NotEmpty(t, fields)

	knownField := fields[0].Name

	config := map[string]any{
		knownField:      "value",
		"legacyPyright": true,
		"fixFlags":      map[string]any{"responseRequiredSep2024": true},
	}

	var warnings []string
	logger := log.New().WithWriter(io.Discard).WithWarnCapture(&warnings)

	ValidateTarget("go", config, false, logger)

	joined := strings.Join(warnings, "\n")
	assert.Contains(t, joined, "legacyPyright")
	assert.Contains(t, joined, "fixFlags")
	assert.NotContains(t, joined, knownField)
}

func TestValidateTarget_NoWarningsForKnownFields(t *testing.T) {
	t.Parallel()

	target, err := generate.GetTargetFromTargetString("go")
	require.NoError(t, err)

	fields, err := generate.GetLanguageConfigFields(target, false)
	require.NoError(t, err)
	require.NotEmpty(t, fields)

	config := map[string]any{fields[0].Name: "value"}

	var warnings []string
	logger := log.New().WithWriter(io.Discard).WithWarnCapture(&warnings)

	ValidateTarget("go", config, false, logger)

	assert.Empty(t, warnings)
}
