package transform

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/index"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/speakeasy-api/speakeasy/internal/log"
)

func RemoveUnused(ctx context.Context, schemaPath string, w io.Writer) error {
	return transformer[interface{}]{
		schemaPath:  schemaPath,
		transformFn: RemoveOrphans,
		w:           w,
	}.Do(ctx)
}

func RemoveOrphans(ctx context.Context, doc libopenapi.Document, model *libopenapi.DocumentModel[v3.Document], _ interface{}) (libopenapi.Document, *libopenapi.DocumentModel[v3.Document], error) {
	logger := log.From(ctx)
	_, doc, model, errs := doc.RenderAndReload()
	// remove nil errs
	var nonNilErrs []error
	for _, e := range errs {
		if e != nil {
			nonNilErrs = append(nonNilErrs, e)
		}
	}
	if len(nonNilErrs) > 0 {
		return nil, nil, fmt.Errorf("failed to render and reload document: %v", errs)
	}

	components := model.Model.Components
	context := model
	allRefs := context.Index.GetAllReferences()
	schemasIdx := context.Index.GetAllComponentSchemas()
	responsesIdx := context.Index.GetAllResponses()
	parametersIdx := context.Index.GetAllParameters()
	examplesIdx := context.Index.GetAllExamples()
	requestBodiesIdx := context.Index.GetAllRequestBodies()
	headersIdx := context.Index.GetAllHeaders()
	securitySchemesIdx := context.Index.GetAllSecuritySchemes()
	linksIdx := context.Index.GetAllLinks()
	callbacksIdx := context.Index.GetAllCallbacks()
	mappedRefs := context.Index.GetMappedReferences()

	checkOpenAPISecurity := func(key string) bool {
		if strings.Contains(key, "securitySchemes") {
			segs := strings.Split(key, "/")
			def := segs[len(segs)-1]
			for r := range context.Index.GetSecurityRequirementReferences() {
				if r == def {
					return true
				}
			}
		}
		return false
	}

	// create poly maps.
	oneOfRefs := make(map[string]*index.Reference)
	allOfRefs := make(map[string]*index.Reference)
	anyOfRefs := make(map[string]*index.Reference)

	// include all polymorphic references.
	for _, ref := range context.Index.GetPolyAllOfReferences() {
		allOfRefs[ref.Definition] = ref
	}
	for _, ref := range context.Index.GetPolyOneOfReferences() {
		oneOfRefs[ref.Definition] = ref
	}
	for _, ref := range context.Index.GetPolyAnyOfReferences() {
		anyOfRefs[ref.Definition] = ref
	}

	notUsed := make(map[string]*index.Reference)
	mapsToSearch := []map[string]*index.Reference{
		schemasIdx,
		responsesIdx,
		parametersIdx,
		examplesIdx,
		requestBodiesIdx,
		headersIdx,
		securitySchemesIdx,
		linksIdx,
		callbacksIdx,
	}

	for _, resultMap := range mapsToSearch {
		for key, ref := range resultMap {

			u := strings.Split(key, "#/")
			keyAlt := key
			if len(u) == 2 {
				if u[0] == "" {
					keyAlt = fmt.Sprintf("%s#/%s", context.Index.GetSpecAbsolutePath(), u[1])
				}
			}

			if allRefs[key] == nil && allRefs[keyAlt] == nil {
				found := false

				if oneOfRefs[key] != nil || allOfRefs[key] != nil || anyOfRefs[key] != nil {
					found = true
				}

				if mappedRefs[key] != nil || mappedRefs[keyAlt] != nil {
					found = true
				}

				if !found {
					found = checkOpenAPISecurity(key)
				}

				if !found {
					notUsed[key] = ref
				}
			}
		}
	}

	// let's start killing orphans
	anyRemoved := false
	schemas := components.Schemas
	toDelete := make([]string, 0)
	for pair := orderedmap.First(schemas); pair != nil; pair = pair.Next() {
		// remove all schemas that are not referenced
		if !isReferenced(pair.Key(), "schemas", notUsed) {
			toDelete = append(toDelete, pair.Key())
			logger.Printf("dropped #/components/schemas/%s\n", pair.Key())
			anyRemoved = true
		}
	}
	for _, key := range toDelete {
		schemas.Delete(key)
	}

	responses := components.Responses
	toDelete = make([]string, 0)
	for pair := orderedmap.First(responses); pair != nil; pair = pair.Next() {
		// remove all responses that are not referenced
		if !isReferenced(pair.Key(), "responses", notUsed) {
			toDelete = append(toDelete, pair.Key())
			logger.Printf("dropped #/components/responses/%s\n", pair.Key())
			anyRemoved = true
		}
	}
	for _, key := range toDelete {
		responses.Delete(key)
	}
	parameters := components.Parameters
	toDelete = make([]string, 0)
	for pair := orderedmap.First(parameters); pair != nil; pair = pair.Next() {
		// remove all parameters that are not referenced
		if !isReferenced(pair.Key(), "parameters", notUsed) {
			toDelete = append(toDelete, pair.Key())
			anyRemoved = true
		}
	}
	for _, key := range toDelete {
		parameters.Delete(key)
	}
	examples := components.Examples
	toDelete = make([]string, 0)
	for pair := orderedmap.First(examples); pair != nil; pair = pair.Next() {
		// remove all examples that are not referenced
		if !isReferenced(pair.Key(), "examples", notUsed) {
			toDelete = append(toDelete, pair.Key())
			anyRemoved = true
		}
	}
	for _, key := range toDelete {
		examples.Delete(key)
	}
	requestBodies := components.RequestBodies
	toDelete = make([]string, 0)
	for pair := orderedmap.First(requestBodies); pair != nil; pair = pair.Next() {
		// remove all requestBodies that are not referenced
		if !isReferenced(pair.Key(), "requestBodies", notUsed) {
			toDelete = append(toDelete, pair.Key())
			anyRemoved = true
		}
	}
	for _, key := range toDelete {
		requestBodies.Delete(key)
	}

	headers := components.Headers
	toDelete = make([]string, 0)
	for pair := orderedmap.First(headers); pair != nil; pair = pair.Next() {
		// remove all headers that are not referenced
		if !isReferenced(pair.Key(), "headers", notUsed) {
			toDelete = append(toDelete, pair.Key())
			anyRemoved = true
		}
	}
	for _, key := range toDelete {
		headers.Delete(key)
	}
	if anyRemoved {
		return RemoveOrphans(ctx, doc, model, nil)
	}
	return doc, model, nil
}

func isReferenced(key string, within string, notUsed map[string]*index.Reference) bool {
	ref := fmt.Sprintf("#/components/%s/%s", within, key)
	if notUsed[ref] != nil {
		return false
	}
	return true
}
