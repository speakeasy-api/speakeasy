package suggest

import (
	"context"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"github.com/speakeasy-api/speakeasy-core/suggestions"
	"github.com/speakeasy-api/speakeasy/internal/schemas"
)

func Diagnose(ctx context.Context, schemaPath string) (suggestions.Diagnosis, error) {
	data, _, _, err := schemas.LoadDocument(ctx, schemaPath)
	if err != nil {
		return nil, err
	}
	summary, err := openapi.GetOASSummary(data, schemaPath)
	if err != nil {
		return nil, err
	}

	return suggestions.Diagnose(ctx, *summary)
}

func ShouldSuggest(d suggestions.Diagnosis) bool {
	return len(d) > 0
}
