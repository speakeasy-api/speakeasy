package suggest

import (
	"context"
	"fmt"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"github.com/speakeasy-api/speakeasy-core/suggestions"
	"github.com/speakeasy-api/speakeasy/internal/schema"
	"strings"
)

func Diagnose(ctx context.Context, schemaPath string) (*suggestions.Diagnosis, error) {
	data, _, _, err := schema.LoadDocument(ctx, schemaPath)
	if err != nil {
		return nil, err
	}
	summary, err := openapi.GetOASSummary(data)
	if err != nil {
		return nil, err
	}

	return suggestions.Diagnose(ctx, *summary)
}
func Summarize(d suggestions.Diagnosis) string {
	var parts []string
	if len(d.InconsistentCasing) > 0 {
		parts = append(parts, fmt.Sprintf("%d operationIDs have inconsistent casing", len(d.InconsistentCasing)))
	}
	if len(d.MissingTags) > 0 {
		parts = append(parts, fmt.Sprintf("%d operations are missing tags", len(d.MissingTags)))
	}
	if len(d.DuplicateInformation) > 0 {
		parts = append(parts, fmt.Sprintf("%d operationIDs contain duplicate information", len(d.DuplicateInformation)))
	}
	if len(d.InconsistentTags) > 0 {
		parts = append(parts, fmt.Sprintf("%d tags have inconsistent casing", len(d.InconsistentTags)))
	}

	if len(parts) == 0 {
		return "Your schema is spotless! No issues found"
	}

	return strings.Join(parts, ", ")
}

func ShouldSuggest(d suggestions.Diagnosis) bool {
	return len(d.InconsistentCasing) > 0 || len(d.MissingTags) > 0 || len(d.DuplicateInformation) > 0 || len(d.InconsistentTags) > 0
}
