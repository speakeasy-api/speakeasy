package modifications

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	lo "github.com/samber/lo"
	"github.com/speakeasy-api/speakeasy-core/suggestions"
	"gopkg.in/yaml.v3"

	"github.com/hashicorp/go-version"
	"github.com/speakeasy-api/openapi-overlay/pkg/loader"
	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
)

const (
	overlayTitle = "Speakeasy Modifications"
	OverlayPath  = ".speakeasy/speakeasy-modifications-overlay.yaml"
)

func GetOverlayPath(dir string) (string, error) {
	// Look for an unused filename for writing the overlay
	overlayPath := filepath.Join(dir, OverlayPath)
	for i := 0; i < 100; i++ {
		if _, err := os.Stat(overlayPath); os.IsNotExist(err) {
			break
		}

		// Remove the .yaml suffix and add a number
		overlayPath = filepath.Join(dir, fmt.Sprintf("%s-%d.yaml", OverlayPath[:len(OverlayPath)-5], i+1))
	}

	relativeOverlayPath, err := filepath.Rel(dir, overlayPath)
	if err != nil {
		return "", fmt.Errorf("error getting relative path: %w", err)
	}

	return relativeOverlayPath, nil
}

func UpsertOverlay(overlayPath string, source *workflow.Source, o overlay.Overlay) (string, error) {
	var overlayFile *os.File
	defer overlayFile.Close()
	var baseOverlay *overlay.Overlay

	_, err := os.Stat(overlayPath)
	// If the file exists, load the current overlay
	if err == nil {
		baseOverlay, err = loader.LoadOverlay(overlayPath)
		if err != nil {
			return overlayPath, err
		}
	} else if os.IsNotExist(err) {
		baseOverlay = &overlay.Overlay{
			Version: "1.0.0",
			Info: overlay.Info{
				Title:   overlayTitle,
				Version: "0.0.0", // bumped later
				Extensions: overlay.Extensions{
					"x-speakeasy-metadata": suggestions.ModificationExtension{
						Type: "speakeasy-modifications",
					},
				},
			},
		}
	} else {
		return overlayPath, err
	}

	allActions := append(baseOverlay.Actions, o.Actions...)
	baseOverlay.Actions = MergeActions(allActions)
	baseOverlay.Info.Version = bumpVersion(baseOverlay.Info.Version, o.Info.Version)

	// Now open it and truncate the existing contents
	overlayFile, err = os.Create(overlayPath)
	if err != nil {
		return overlayPath, fmt.Errorf("error opening existing overlay file: %w", err)
	}

	UpsertOverlayIntoSource(source, overlayPath)
	return overlayPath, baseOverlay.Format(overlayFile)
}

func UpsertOverlayIntoSource(source *workflow.Source, overlayPath string) {
	if source == nil {
		return
	}

	// Add the new overlay to the source, if not already present
	if !slices.ContainsFunc(source.Overlays, func(o workflow.Overlay) bool { return o.Document.Location.Reference() == overlayPath }) {
		source.Overlays = append(source.Overlays, workflow.Overlay{
			Document: &workflow.Document{
				Location: workflow.LocationString(overlayPath),
			},
		})
	}
}

type actionAndModification struct {
	action overlay.Action
	m      suggestions.ModificationExtension
}

// Keep the first action for each target and modification type
func MergeActions(actions []overlay.Action) []overlay.Action {
	seen := map[string][]*actionAndModification{}
	var deduped []overlay.Action

	for _, action := range actions {
		newModification := suggestions.GetModificationExtension(action)
		if newModification == nil {
			deduped = append(deduped, action)
			continue
		}

		// If we've already seen a modification of this type, merge the reviewedAt and disabled fields
		if i := slices.IndexFunc(seen[action.Target], func(a *actionAndModification) bool {
			return a.m.Type == newModification.Type
		}); i != -1 {
			existingAction := seen[action.Target][i]

			// We get here when changes have been made in the UI, such as reviewing or disabling a suggestion
			if newModification.ReviewedAt != 0 {
				existingAction.m.ReviewedAt = newModification.ReviewedAt
			}
			if newModification.Disabled {
				existingAction.m.Disabled = newModification.Disabled
				existingAction.action.Update = yaml.Node{} // This makes the action a no-op, disabling it without removing it
			}
		} else {
			// Otherwise, add the modification
			seen[action.Target] = append(seen[action.Target], &actionAndModification{action, *newModification})
			// Don't add it to the deduped list until we're done editing it
		}
	}

	for _, actions := range seen {
		for _, action := range actions {
			// Update with the merged modification extension
			action.action.Extensions["x-speakeasy-metadata"] = action.m
			deduped = append(deduped, action.action)
		}
	}

	return deduped
}

// Return new suggestions that are not already in the list of suggestions
func RemoveAlreadySuggested(alreadySuggested []overlay.Action, newSuggestions []overlay.Action) []overlay.Action {
	getKey := func(x overlay.Action, index int) string {
		if mod := suggestions.GetModificationExtension(x); mod != nil {
			return mod.Type + ":" + x.Target
		}
		return ":" + x.Target
	}

	alreadySuggestedKeys := lo.Map(alreadySuggested, getKey)

	return lo.Filter(newSuggestions, func(x overlay.Action, index int) bool {
		key := getKey(x, index)
		return slices.Contains(alreadySuggestedKeys, key)
	})
}

// If the new version is greater than the base version, return the new version
// If the new version is less than the base version, return the base version with the patch incremented
func bumpVersion(baseVersion, newVersion string) (v string) {
	v = "0.0.1"

	baseSemver, err := version.NewSemver(baseVersion)
	if err != nil {
		return
	}

	newSemver, err := version.NewSemver(baseVersion)
	if err != nil {
		return
	}

	if newSemver.GreaterThan(baseSemver) {
		v = newVersion
	} else {
		segments := baseSemver.Segments()
		segments[2]++

		v = ""
		for i, segment := range segments {
			if i > 0 {
				v += "."
			}
			v += strconv.Itoa(segment)
		}
	}

	return
}
