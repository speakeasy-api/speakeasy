package merge

import (
	"fmt"
	"strings"

	"github.com/speakeasy-api/openapi/openapi"
)

// mergeState tracks namespace provenance across sequential document merges.
// It enables case-insensitive tag dedup, path/method conflict detection,
// and operationId deduplication.
type mergeState struct {
	// tagTracker maps lowercase tag name → list of entries tracking each
	// distinct tag with that name (possibly suffixed).
	tagTracker map[string][]tagEntry

	// opTracker maps "path|method" → provenance of the document that contributed it.
	opTracker map[string]opProvenance

	// opIdTracker maps operationId → list of locations using that id.
	opIdTracker map[string][]opIdEntry
}

// tagEntry tracks a single tag instance across merges.
type tagEntry struct {
	currentName string // the current name in the merged doc (may be suffixed)
	namespace   string // namespace of the document that contributed this tag
	suffixed    bool   // whether this entry has already been disambiguated
}

// opProvenance records which document contributed an operation.
type opProvenance struct {
	namespace string
	counter   int
}

// opIdEntry tracks a single operationId occurrence.
type opIdEntry struct {
	path      string
	method    openapi.HTTPMethod
	namespace string
}

func newMergeState() *mergeState {
	return &mergeState{
		tagTracker:  make(map[string][]tagEntry),
		opTracker:   make(map[string]opProvenance),
		opIdTracker: make(map[string][]opIdEntry),
	}
}

// initMergeState registers the first document's tags and operations into the state.
func initMergeState(state *mergeState, doc *openapi.OpenAPI, namespace string) {
	// Register tags
	for _, tag := range doc.Tags {
		key := strings.ToLower(tag.Name)
		state.tagTracker[key] = append(state.tagTracker[key], tagEntry{
			currentName: tag.Name,
			namespace:   namespace,
		})
	}

	// Register path operations
	if doc.Paths != nil {
		for path, pathItem := range doc.Paths.All() {
			if pathItem == nil || pathItem.Object == nil {
				continue
			}
			for method, op := range pathItem.Object.All() {
				registerOp(state, path, method, namespace, 1, op)
			}
		}
	}
}

// registerOp registers an operation in both the opTracker and opIdTracker.
func registerOp(state *mergeState, path string, method openapi.HTTPMethod, namespace string, counter int, op *openapi.Operation) {
	pmKey := pathMethodKey(path, method)
	state.opTracker[pmKey] = opProvenance{namespace: namespace, counter: counter}

	if op != nil && op.OperationID != nil && *op.OperationID != "" {
		opId := *op.OperationID
		// Remove any existing entry for this path+method to avoid duplicates
		// when an operation is overwritten during a "same content, last wins" merge.
		entries := state.opIdTracker[opId]
		for i, e := range entries {
			if e.path == path && e.method == method {
				state.opIdTracker[opId] = append(entries[:i], entries[i+1:]...)
				break
			}
		}
		state.opIdTracker[opId] = append(state.opIdTracker[opId], opIdEntry{
			path:      path,
			method:    method,
			namespace: namespace,
		})
	}
}

// unregisterOp removes an operation's opIdTracker entry for a specific path+method.
// Used when an operation is moved from its original path to a fragment path.
func unregisterOp(state *mergeState, path string, method openapi.HTTPMethod, op *openapi.Operation) {
	if op == nil || op.OperationID == nil || *op.OperationID == "" {
		return
	}
	opId := *op.OperationID
	entries := state.opIdTracker[opId]
	filtered := make([]opIdEntry, 0, len(entries))
	for _, e := range entries {
		if e.path == path && e.method == method {
			continue
		}
		filtered = append(filtered, e)
	}
	state.opIdTracker[opId] = filtered
}

// pathMethodKey builds a unique key for a path+method combination.
func pathMethodKey(path string, method openapi.HTTPMethod) string {
	return path + "|" + string(method)
}

// disambiguatingSuffix returns the namespace string if non-empty,
// otherwise returns the counter formatted as a string.
func disambiguatingSuffix(namespace string, counter int) string {
	if namespace != "" {
		return namespace
	}
	return fmt.Sprintf("%d", counter)
}
