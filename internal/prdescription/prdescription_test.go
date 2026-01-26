package prdescription

import (
	"strings"
	"testing"

	"github.com/speakeasy-api/versioning-reports/versioning"
	"github.com/stretchr/testify/assert"
)

func TestGenerate_BasicSDKUpdate(t *testing.T) {
	input := Input{
		WorkflowName:     "Generate",
		SourceBranch:     "main",
		SpeakeasyVersion: "1.234.0",
	}

	output, err := Generate(input)
	assert.NoError(t, err)
	assert.Equal(t, "chore: Update SDK - Generate", output.Title)
	assert.Contains(t, output.Body, "# SDK update")
	assert.Contains(t, output.Body, "Based on [Speakeasy CLI]")
}

func TestGenerate_WithReportURLs(t *testing.T) {
	input := Input{
		LintingReportURL: "https://example.com/linting",
		ChangesReportURL: "https://example.com/changes",
		WorkflowName:     "Generate",
		SpeakeasyVersion: "1.234.0",
	}

	output, err := Generate(input)
	assert.NoError(t, err)
	assert.Contains(t, output.Body, "> [!IMPORTANT]")
	assert.Contains(t, output.Body, "Linting report available at: <https://example.com/linting>")
	assert.Contains(t, output.Body, "OpenAPI Change report available at: <https://example.com/changes>")
}

func TestGenerate_WithFeatureBranch(t *testing.T) {
	input := Input{
		WorkflowName:     "Generate",
		SourceBranch:     "main",
		FeatureBranch:    "feature/new-api",
		SpeakeasyVersion: "1.234.0",
	}

	output, err := Generate(input)
	assert.NoError(t, err)
	assert.Equal(t, "chore: Update SDK - Generate [feature/new-api]", output.Title)
}

func TestGenerate_WithNonMainSourceBranch(t *testing.T) {
	input := Input{
		WorkflowName:     "Generate",
		SourceBranch:     "refs/heads/develop",
		SpeakeasyVersion: "1.234.0",
	}

	output, err := Generate(input)
	assert.NoError(t, err)
	assert.Equal(t, "chore: Update SDK - Generate [develop]", output.Title)
}

func TestGenerate_SourceGeneration(t *testing.T) {
	input := Input{
		WorkflowName:     "Generate",
		SourceGeneration: true,
	}

	output, err := Generate(input)
	assert.NoError(t, err)
	assert.Equal(t, "chore: Update Specs - Generate", output.Title)
	assert.Contains(t, output.Body, "Update of compiled sources")
	assert.NotContains(t, output.Body, "Based on [Speakeasy CLI]")
}

func TestGenerate_DocsGeneration(t *testing.T) {
	input := Input{
		WorkflowName:   "Generate",
		DocsGeneration: true,
	}

	output, err := Generate(input)
	assert.NoError(t, err)
	assert.Equal(t, "chore: Update SDK Docs - Generate", output.Title)
}

func TestGenerate_WithVersionReport(t *testing.T) {
	input := Input{
		WorkflowName:     "Generate",
		SpeakeasyVersion: "1.234.0",
		VersionReport: &versioning.MergedVersionReport{
			Reports: []versioning.VersionReport{
				{
					Key:        "SDK_CHANGELOG_typescript",
					BumpType:   versioning.BumpPatch,
					NewVersion: "0.5.1",
					PRReport:   "## Typescript SDK Changes:\n* Some change",
				},
			},
		},
	}

	output, err := Generate(input)
	assert.NoError(t, err)
	assert.Equal(t, "chore: Update SDK - Generate 0.5.1", output.Title)
	assert.Contains(t, output.Body, "Version Bump Type: [patch]")
	assert.Contains(t, output.Body, "(automated)")
	assert.Contains(t, output.Body, "## Typescript SDK Changes:")
}

func TestGenerate_ManualBump(t *testing.T) {
	input := Input{
		WorkflowName:     "Generate",
		SpeakeasyVersion: "1.234.0",
		ManualBump:       true,
		VersionReport: &versioning.MergedVersionReport{
			Reports: []versioning.VersionReport{
				{
					BumpType:   versioning.BumpMinor,
					NewVersion: "0.6.0",
				},
			},
		},
	}

	output, err := Generate(input)
	assert.NoError(t, err)
	assert.Contains(t, output.Body, "**Version Bump Type: [minor]")
	assert.Contains(t, output.Body, "(manual)**")
	assert.Contains(t, output.Body, "until the minor label is removed")
}

func TestGenerate_MultipleTargets_NoVersionInTitle(t *testing.T) {
	input := Input{
		WorkflowName:     "Generate",
		SpeakeasyVersion: "1.234.0",
		VersionReport: &versioning.MergedVersionReport{
			Reports: []versioning.VersionReport{
				{NewVersion: "0.5.1"},
				{NewVersion: "0.6.0"}, // Different version
			},
		},
	}

	output, err := Generate(input)
	assert.NoError(t, err)
	// Multiple different versions should not include version in title
	assert.Equal(t, "chore: Update SDK - Generate", output.Title)
}

func TestGenerate_SpecifiedTarget(t *testing.T) {
	input := Input{
		WorkflowName:    "Generate",
		SpecifiedTarget: "python",
	}

	output, err := Generate(input)
	assert.NoError(t, err)
	assert.Equal(t, "chore: Update SDK - Generate PYTHON", output.Title)
}

func TestStripANSICodes(t *testing.T) {
	input := "\x1b[32mgreen text\x1b[0m and \x1b[1;31mred bold\x1b[0m"
	expected := "green text and red bold"
	assert.Equal(t, expected, stripANSICodes(input))
}

func TestIsMainBranch(t *testing.T) {
	assert.True(t, isMainBranch("main"))
	assert.True(t, isMainBranch("Main"))
	assert.True(t, isMainBranch("master"))
	assert.True(t, isMainBranch("MASTER"))
	assert.False(t, isMainBranch("develop"))
	assert.False(t, isMainBranch("feature/test"))
}

func TestSanitizeBranchName(t *testing.T) {
	assert.Equal(t, "develop", sanitizeBranchName("refs/heads/develop"))
	assert.Equal(t, "feature/test", sanitizeBranchName("refs/heads/feature/test"))
	assert.Equal(t, "main", sanitizeBranchName("main"))
}

func TestGenerate_EmptyInput(t *testing.T) {
	input := Input{}

	output, err := Generate(input)
	assert.NoError(t, err)
	assert.Equal(t, "chore: Update SDK - ", output.Title)
	assert.True(t, strings.HasPrefix(output.Body, "# SDK update"))
}
