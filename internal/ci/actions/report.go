package actions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/speakeasy-api/speakeasy/internal/ci/environment"
	"github.com/speakeasy-api/speakeasy/internal/ci/logging"
	"github.com/speakeasy-api/versioning-reports/versioning"
)

// TargetGenerationReport captures all CI-agnostic data from a single target's
// generation run that is needed to build a PR description later.
type TargetGenerationReport struct {
	Target               string                          `json:"target"`
	VersionReport        *versioning.MergedVersionReport `json:"version_report,omitempty"`
	LintingReportURL     string                          `json:"linting_report_url,omitempty"`
	ChangesReportURL     string                          `json:"changes_report_url,omitempty"`
	OpenAPIChangeSummary string                          `json:"openapi_change_summary,omitempty"`
	SpeakeasyVersion     string                          `json:"speakeasy_version,omitempty"`
	ManualBump           bool                            `json:"manual_bump,omitempty"`
}

const reportsDir = ".speakeasy/reports"

// writeGenerationReport serializes a TargetGenerationReport to
// .speakeasy/reports/<target>.json and returns the file path.
// Only writes when a specific target is set (matrix mode).
func writeGenerationReport(report TargetGenerationReport) (string, error) {
	if environment.GetMode() != environment.ModeMatrix {
		return "", nil
	}

	target := environment.SpecifiedTarget()
	if target == "" {
		return "", nil
	}

	report.Target = target

	if err := os.MkdirAll(reportsDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create reports directory: %w", err)
	}

	reportPath := filepath.Join(reportsDir, target+".json")

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal generation report: %w", err)
	}

	if err := os.WriteFile(reportPath, data, 0o644); err != nil {
		return "", fmt.Errorf("failed to write generation report: %w", err)
	}

	logging.Info("Wrote generation report to %s", reportPath)

	return reportPath, nil
}
