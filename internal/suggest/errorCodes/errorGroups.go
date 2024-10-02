package errorCodes

import (
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"slices"
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
			codes:        []string{"400", "413", "414", "415", "422", "431", "510"},
			description:  "A collection of codes that generally means the end user got something wrong in making the request",
			schemaName:   "BadRequestError",
			responseName: "BadRequest",
		},
		{
			name:         "Unauthorized",
			codes:        []string{"401", "403", "407", "511"},
			description:  "A collection of codes that generally means the client was not authenticated correctly for the request they want to make",
			schemaName:   "UnauthorizedError",
			responseName: "Unauthorized",
		},
		{
			name:         "NotFound",
			codes:        []string{"404", "501", "505"},
			description:  "Status codes relating to the resource/entity they are requesting not being found or endpoints/routes not existing",
			schemaName:   "NotFoundError",
			responseName: "NotFound",
		},
		{
			name:         "RateLimited",
			codes:        []string{"429"},
			description:  "Status codes relating to the client being rate limited by the server",
			schemaName:   "RateLimitedError",
			responseName: "RateLimited",
		},
		{
			name:         "InternalServerError",
			codes:        []string{"500", "502", "503", "506", "507", "508"},
			description:  "A collection of status codes that generally mean the server failed in an unexpected way",
			schemaName:   "InternalServerError",
			responseName: "InternalServerError",
		},
		{
			name:         "Timeout",
			codes:        []string{"408", "504"},
			description:  "Timeouts occurred with the request",
			schemaName:   "TimeoutError",
			responseName: "Timeout",
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
