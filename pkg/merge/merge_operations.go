package merge

import (
	"github.com/speakeasy-api/openapi/openapi"
	"github.com/speakeasy-api/openapi/sequencedmap"
)

// mergePathsWithState merges paths from doc into mergedDoc, detecting method-level
// conflicts and creating fragment paths (path#suffix) for disambiguation.
func mergePathsWithState(state *mergeState, mergedDoc, doc *openapi.OpenAPI, docNamespace string, docCounter int) []error {
	errs := make([]error, 0)

	if doc.Paths == nil {
		return errs
	}

	if mergedDoc.Paths == nil {
		mergedDoc.Paths = doc.Paths
		// Register all operations from the first paths
		for path, pathItem := range doc.Paths.All() {
			if pathItem == nil || pathItem.Object == nil {
				continue
			}
			for method, op := range pathItem.Object.All() {
				registerOp(state, path, method, docNamespace, docCounter, op)
			}
		}
		return errs
	}

	// Merge path-level extensions
	var extensionErr []error
	mergedDoc.Paths.Extensions, extensionErr = mergeExtensions(mergedDoc.Paths.Extensions, doc.Paths.Extensions)
	errs = append(errs, extensionErr...)

	for path, pathItem := range doc.Paths.All() {
		mergedPathItem, exists := mergedDoc.Paths.Get(path)
		if !exists {
			// New path — add it and register operations
			mergedDoc.Paths.Set(path, pathItem)
			if pathItem != nil && pathItem.Object != nil {
				for method, op := range pathItem.Object.All() {
					registerOp(state, path, method, docNamespace, docCounter, op)
				}
			}
			continue
		}

		if pathItem == nil || pathItem.Object == nil {
			continue
		}
		if mergedPathItem == nil || mergedPathItem.Object == nil {
			mergedDoc.Paths.Set(path, pathItem)
			if pathItem.Object != nil {
				for method, op := range pathItem.Object.All() {
					registerOp(state, path, method, docNamespace, docCounter, op)
				}
			}
			continue
		}

		// Path exists in both — check for method-level conflicts
		var conflictMethods []openapi.HTTPMethod

		for method, incomingOp := range pathItem.Object.All() {
			existingOp := mergedPathItem.Object.GetOperation(method)
			if existingOp == nil {
				continue
			}
			// Compare operations
			if err := isReferencedEquivalent(existingOp, incomingOp); err != nil {
				// Different content — this method conflicts
				conflictMethods = append(conflictMethods, method)
			}
		}

		if len(conflictMethods) == 0 {
			// No conflicts — normal merge
			pi, pathItemErrs := mergePathItemObjects(mergedPathItem.Object, pathItem.Object)
			mergedPathItem.Object = pi
			mergedDoc.Paths.Set(path, mergedPathItem)
			errs = append(errs, pathItemErrs...)
			// Register new operations
			for method, op := range pathItem.Object.All() {
				registerOp(state, path, method, docNamespace, docCounter, op)
			}
			continue
		}

		// Has conflicts — handle conflicting and non-conflicting methods separately
		conflictSet := make(map[openapi.HTTPMethod]bool, len(conflictMethods))
		for _, m := range conflictMethods {
			conflictSet[m] = true
		}

		// Merge non-conflicting methods into the existing path item
		for method, op := range pathItem.Object.All() {
			if conflictSet[method] {
				continue
			}
			mergedPathItem.Object.Set(method, op)
			registerOp(state, path, method, docNamespace, docCounter, op)
		}

		// Also merge non-operation fields from the incoming path item
		if pathItem.Object.Summary != nil && *pathItem.Object.Summary != "" {
			mergedPathItem.Object.Summary = pathItem.Object.Summary
		}
		if pathItem.Object.Description != nil && *pathItem.Object.Description != "" {
			mergedPathItem.Object.Description = pathItem.Object.Description
		}
		mergedPathItem.Object.Parameters = mergeParameters(mergedPathItem.Object.Parameters, pathItem.Object.Parameters)
		mergedPathItem.Object.Servers, _ = mergeServers(mergedPathItem.Object.Servers, pathItem.Object.Servers, false)
		if pathItem.Object.Extensions != nil {
			var extErrs []error
			mergedPathItem.Object.Extensions, extErrs = mergeExtensions(mergedPathItem.Object.Extensions, pathItem.Object.Extensions)
			errs = append(errs, extErrs...)
		}

		// Handle conflicting methods by creating fragment paths
		for _, method := range conflictMethods {
			existingOp := mergedPathItem.Object.GetOperation(method)
			incomingOp := pathItem.Object.GetOperation(method)

			existing := state.opTracker[pathMethodKey(path, method)]

			// Unregister the existing operation from its original path before moving
			unregisterOp(state, path, method, existingOp)

			// Move existing operation to a fragment path
			existingSuffix := disambiguatingSuffix(existing.namespace, existing.counter)
			existingFragPath := path + "#" + existingSuffix
			if _, alreadyMoved := mergedDoc.Paths.Get(existingFragPath); !alreadyMoved {
				existingFragItem := openapi.NewPathItem()
				existingFragItem.Set(method, existingOp)
				mergedDoc.Paths.Set(existingFragPath, openapi.NewReferencedPathItemFromPathItem(existingFragItem))
				registerOp(state, existingFragPath, method, existing.namespace, existing.counter, existingOp)
			}

			// Create fragment path for incoming operation
			incomingSuffix := disambiguatingSuffix(docNamespace, docCounter)
			incomingFragPath := path + "#" + incomingSuffix
			incomingFragItem := openapi.NewPathItem()
			incomingFragItem.Set(method, incomingOp)
			mergedDoc.Paths.Set(incomingFragPath, openapi.NewReferencedPathItemFromPathItem(incomingFragItem))
			registerOp(state, incomingFragPath, method, docNamespace, docCounter, incomingOp)

			// Remove conflicting method from original path
			mergedPathItem.Object.Map.Delete(method)
		}

		// If the original path item is now empty of operations, remove it
		if mergedPathItem.Object.Len() == 0 {
			mergedDoc.Paths.Delete(path)
		}
	}

	return errs
}

// deduplicateOperationIds performs a post-merge pass to suffix duplicate operationIds.
func deduplicateOperationIds(state *mergeState, doc *openapi.OpenAPI) {
	// Find which operationIds have duplicates
	duplicates := make(map[string]bool)
	for opId, entries := range state.opIdTracker {
		if len(entries) > 1 {
			duplicates[opId] = true
		}
	}

	if len(duplicates) == 0 {
		return
	}

	// Walk all operations and suffix duplicates
	deduplicateOpsInPaths(state, doc.Paths, duplicates)
	deduplicateOpsInWebhooks(state, doc.Webhooks, duplicates)
}

func deduplicateOpsInPaths(state *mergeState, paths *openapi.Paths, duplicates map[string]bool) {
	if paths == nil {
		return
	}

	for path, pathItem := range paths.All() {
		if pathItem == nil || pathItem.Object == nil {
			continue
		}
		for method, op := range pathItem.Object.All() {
			deduplicateOp(state, op, path, method, duplicates)
		}
	}
}

func deduplicateOpsInWebhooks(state *mergeState, webhooks *sequencedmap.Map[string, *openapi.ReferencedPathItem], duplicates map[string]bool) {
	if webhooks == nil {
		return
	}

	for path, pathItem := range webhooks.All() {
		if pathItem == nil || pathItem.Object == nil {
			continue
		}
		for method, op := range pathItem.Object.All() {
			deduplicateOp(state, op, path, method, duplicates)
		}
	}
}

func deduplicateOp(state *mergeState, op *openapi.Operation, path string, method openapi.HTTPMethod, duplicates map[string]bool) {
	if op == nil || op.OperationID == nil || *op.OperationID == "" {
		return
	}

	opId := *op.OperationID
	if !duplicates[opId] {
		return
	}

	entries := state.opIdTracker[opId]

	// Find which entry matches this operation's location
	for i, entry := range entries {
		if entry.path == path && entry.method == method {
			suffix := disambiguatingSuffix(entry.namespace, i+1)
			newId := opId + "_" + suffix
			op.OperationID = &newId
			return
		}
	}
}
