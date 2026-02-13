package merge

import (
	"strings"

	"github.com/speakeasy-api/openapi/openapi"
)

// tagRenameResult holds separate rename maps for the existing (merged) document
// and the incoming document. Keeping them separate allows case-insensitive
// operation-level tag renaming without ambiguity — each map has at most one
// entry per case-insensitive tag name.
type tagRenameResult struct {
	existingRenames map[string]string // renames for operations already in mergedDoc
	incomingRenames map[string]string // renames for operations in the incoming doc
}

// mergeTagsWithState performs case-insensitive tag merging with content-aware disambiguation.
// It returns separate rename maps for updating operation-level tag references in the
// existing merged document and the incoming document.
func mergeTagsWithState(state *mergeState, mergedDoc, doc *openapi.OpenAPI, docNamespace string, docCounter int) tagRenameResult {
	result := tagRenameResult{
		existingRenames: make(map[string]string),
		incomingRenames: make(map[string]string),
	}

	if doc.Tags == nil {
		return result
	}

	if mergedDoc.Tags == nil {
		mergedDoc.Tags = make([]*openapi.Tag, 0, len(doc.Tags))
	}

	for _, newTag := range doc.Tags {
		key := strings.ToLower(newTag.Name)
		entries := state.tagTracker[key]

		if len(entries) == 0 {
			// Completely new tag — add as-is
			mergedDoc.Tags = append(mergedDoc.Tags, newTag)
			state.tagTracker[key] = []tagEntry{{
				currentName: newTag.Name,
				namespace:   docNamespace,
			}}
			continue
		}

		// Find the matching tag in the merged doc by looking up the tracker entries.
		matchIdx, matchEntry := findTagInMergedDoc(mergedDoc, entries)
		if matchIdx < 0 {
			// Tracker says it exists but we can't find it — just append
			mergedDoc.Tags = append(mergedDoc.Tags, newTag)
			state.tagTracker[key] = append(entries, tagEntry{
				currentName: newTag.Name,
				namespace:   docNamespace,
			})
			continue
		}

		existingTag := mergedDoc.Tags[matchIdx]

		if tagsContentEqual(existingTag, newTag) {
			// Same content, only casing may differ — last one wins (replace in-place)
			oldName := existingTag.Name
			mergedDoc.Tags[matchIdx] = newTag
			// Record rename if the name changed (e.g. casing difference)
			if oldName != newTag.Name {
				result.existingRenames[oldName] = newTag.Name
			}
			// Update the tracker entry
			state.tagTracker[key][matchEntry] = tagEntry{
				currentName: newTag.Name,
				namespace:   docNamespace,
				suffixed:    entries[matchEntry].suffixed,
			}
		} else {
			// Different content — disambiguate

			// Suffix the existing tag if not already suffixed
			if !entries[matchEntry].suffixed {
				existingSuffix := disambiguatingSuffix(entries[matchEntry].namespace, findCounterForEntry(entries, matchEntry))
				oldName := existingTag.Name
				newName := oldName + "_" + existingSuffix
				existingTag.Name = newName
				result.existingRenames[oldName] = newName
				state.tagTracker[key][matchEntry] = tagEntry{
					currentName: newName,
					namespace:   entries[matchEntry].namespace,
					suffixed:    true,
				}
			}

			// Suffix the new tag
			newSuffix := disambiguatingSuffix(docNamespace, docCounter)
			oldNewTagName := newTag.Name
			newTag.Name = oldNewTagName + "_" + newSuffix
			result.incomingRenames[oldNewTagName] = newTag.Name

			mergedDoc.Tags = append(mergedDoc.Tags, newTag)
			state.tagTracker[key] = append(state.tagTracker[key], tagEntry{
				currentName: newTag.Name,
				namespace:   docNamespace,
				suffixed:    true,
			})
		}
	}

	return result
}

// findTagInMergedDoc locates the tag in mergedDoc.Tags that corresponds to one
// of the tracker entries. Returns the index in mergedDoc.Tags and the index in entries.
func findTagInMergedDoc(mergedDoc *openapi.OpenAPI, entries []tagEntry) (int, int) {
	for entryIdx, entry := range entries {
		for tagIdx, tag := range mergedDoc.Tags {
			if tag.Name == entry.currentName {
				return tagIdx, entryIdx
			}
		}
	}
	return -1, -1
}

// findCounterForEntry returns a 1-based counter position for the entry,
// used when no namespace is available.
func findCounterForEntry(_ []tagEntry, idx int) int {
	return idx + 1
}

// tagsContentEqual compares two tags ignoring the Name, Description, and
// Summary fields. Returns true if all other fields (ExternalDocs,
// Extensions, Parent, Kind) are equivalent.
func tagsContentEqual(a, b *openapi.Tag) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Compare pointer string fields (skip Summary and Description as they
	// are descriptive-only and shouldn't cause merge conflicts)
	if !ptrStringEqual(a.Parent, b.Parent) {
		return false
	}
	if !ptrStringEqual(a.Kind, b.Kind) {
		return false
	}

	// Compare ExternalDocs
	switch {
	case a.ExternalDocs == nil && b.ExternalDocs == nil:
		// both nil, ok
	case a.ExternalDocs == nil || b.ExternalDocs == nil:
		return false
	default:
		if err := isReferencedEquivalent(a.ExternalDocs, b.ExternalDocs); err != nil {
			return false
		}
	}

	// Compare Extensions
	switch {
	case a.Extensions == nil && b.Extensions == nil:
		// both nil, ok
	case a.Extensions == nil || b.Extensions == nil:
		return false
	default:
		if err := isReferencedEquivalent(a.Extensions, b.Extensions); err != nil {
			return false
		}
	}

	return true
}

// ptrStringEqual compares two *string values.
func ptrStringEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// updateOperationTagRefs walks all operations in the document and updates
// tag name references according to the renames map (case-insensitive).
func updateOperationTagRefs(doc *openapi.OpenAPI, renames map[string]string) {
	if len(renames) == 0 {
		return
	}

	// Build case-insensitive lookup once and reuse for all operations.
	ciRenames := make(map[string]string, len(renames))
	for oldName, newName := range renames {
		ciRenames[strings.ToLower(oldName)] = newName
	}

	if doc.Paths != nil {
		for _, pathItem := range doc.Paths.All() {
			if pathItem == nil || pathItem.Object == nil {
				continue
			}
			for _, op := range pathItem.Object.All() {
				renameOpTags(op, ciRenames)
			}
		}
	}

	if doc.Webhooks != nil {
		for _, pathItem := range doc.Webhooks.All() {
			if pathItem == nil || pathItem.Object == nil {
				continue
			}
			for _, op := range pathItem.Object.All() {
				renameOpTags(op, ciRenames)
			}
		}
	}
}

// renameOpTags updates the Tags slice of a single operation using a
// pre-built case-insensitive rename map (lowercase keys → new names).
func renameOpTags(op *openapi.Operation, ciRenames map[string]string) {
	if op == nil {
		return
	}

	for i, tag := range op.Tags {
		if newName, ok := ciRenames[strings.ToLower(tag)]; ok {
			op.Tags[i] = newName
		}
	}
}

// normalizeOperationTags is a post-merge pass that ensures all operation-level
// tag references use the same casing as the document-level tag definitions.
// This catches cases where operations reference tags with a casing that wasn't
// directly involved in a rename (e.g. "PETS" when the rename only covered
// "Pets" → "pets"), or where tags are used only in operations without any
// document-level definition.
func normalizeOperationTags(doc *openapi.OpenAPI) {
	// Build canonical name map from document-level tags.
	canonical := make(map[string]string, len(doc.Tags))
	for _, tag := range doc.Tags {
		canonical[strings.ToLower(tag.Name)] = tag.Name
	}

	normalize := func(op *openapi.Operation) {
		if op == nil {
			return
		}
		for i, tag := range op.Tags {
			key := strings.ToLower(tag)
			if chosen, ok := canonical[key]; ok {
				op.Tags[i] = chosen
			} else {
				// Tag not defined at document level — use first occurrence as canonical.
				canonical[key] = tag
			}
		}
	}

	if doc.Paths != nil {
		for _, pathItem := range doc.Paths.All() {
			if pathItem == nil || pathItem.Object == nil {
				continue
			}
			for _, op := range pathItem.Object.All() {
				normalize(op)
			}
		}
	}

	if doc.Webhooks != nil {
		for _, pathItem := range doc.Webhooks.All() {
			if pathItem == nil || pathItem.Object == nil {
				continue
			}
			for _, op := range pathItem.Object.All() {
				normalize(op)
			}
		}
	}
}
