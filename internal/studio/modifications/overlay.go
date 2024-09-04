package modifications

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"

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
	// Open the file with read and write permissions
	overlayFile, err := os.OpenFile(overlayPath, os.O_RDWR, 0644)
	var baseOverlay *overlay.Overlay

	// If the file exists, load the current overlay
	if err == nil {
		baseOverlay, err = loader.LoadOverlay(overlayPath)
		if err != nil {
			return overlayPath, err
		}
	} else if os.IsNotExist(err) {
		overlayFile, err = os.Create(overlayPath)
		if err != nil {
			return overlayPath, err
		}

		baseOverlay = &overlay.Overlay{
			Version: "1.0.0",
			Info: overlay.Info{
				Title:   overlayTitle,
				Version: "0.0.0", // bumped later
			},
		}
	} else {
		return overlayPath, err
	}

	baseOverlay.Actions = append(baseOverlay.Actions, o.Actions...)
	baseOverlay.Info.Version = bumpVersion(baseOverlay.Info.Version, o.Info.Version)

	UpsertOverlayIntoSource(source, overlayPath)
	return overlayPath, baseOverlay.Format(overlayFile)
}

func UpsertOverlayIntoSource(source *workflow.Source, overlayPath string) {
	if source == nil {
		return
	}

	// Add the new overlay to the source, if not already present
	if !slices.ContainsFunc(source.Overlays, func(o workflow.Overlay) bool { return o.Document.Location == overlayPath }) {
		source.Overlays = append(source.Overlays, workflow.Overlay{
			Document: &workflow.Document{
				Location: overlayPath,
			},
		})
	}
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
