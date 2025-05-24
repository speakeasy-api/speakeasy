package errorCodes

import (
	"slices"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/speakeasy-api/speakeasy-core/openapi"
)

type errorGroup struct {
	name                     string
	codes                    []string
	description              string
	schemaName, responseName string
}
type errorGroupSlice []errorGroup

func initErrorGroups() errorGroupSlice {
	return errorGroupSlice{
		{
			name:         "BadRequest",
			codes:        []string{"400", "422"},
			description:  "Invalid request",
			schemaName:   "BadRequest",
			responseName: "BadRequest",
		},
		{
			name:         "Unauthorized",
			codes:        []string{"401", "403"},
			description:  "Permission denied or not authenticated",
			schemaName:   "Unauthorized",
			responseName: "Unauthorized",
		},
		{
			name:         "NotFound",
			codes:        []string{"404"},
			description:  "Not found",
			schemaName:   "NotFound",
			responseName: "NotFound",
		},
		{
			name:         "RateLimited",
			codes:        []string{"429"},
			description:  "Rate limit exceeded",
			schemaName:   "RateLimited",
			responseName: "RateLimited",
		},
	}
}

func (e errorGroupSlice) FindCode(code string) errorGroup {
	for _, group := range e {
		if slices.Contains(group.codes, code) {
			return group
		}
	}
	return errorGroup{}
}

func (e errorGroupSlice) AllCodes() []string {
	var codes []string
	for _, group := range e {
		codes = append(codes, group.codes...)
	}
	return codes
}

func (e errorGroupSlice) DeduplicateComponentNames(document v3.Document) {
	var schemaNames []string
	for s := range openapi.IterateSchemas(document) {
		schemaNames = append(schemaNames, s.Name)
	}

	var responseNames []string
	for r := range openapi.IterateResponses(document) {
		responseNames = append(responseNames, r.Name)
	}

	for i, group := range e {
		e[i].responseName = findUnusedName(group.responseName, responseNames)
		e[i].schemaName = findUnusedName(group.schemaName, schemaNames)
	}
}
