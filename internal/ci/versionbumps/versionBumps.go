package versionbumps

import (
	"fmt"
	"regexp"

	"github.com/google/go-github/v63/github"
	"github.com/speakeasy-api/versioning-reports/versioning"
	"golang.org/x/exp/slices"
)

type BumpMethod string

// Enum values for BumpMethod
const (
	BumpMethodManual    BumpMethod = "👤"
	BumpMethodAutomated BumpMethod = "🤖"
)

var bumpTypeLabels = map[versioning.BumpType]string{
	versioning.BumpMajor:      "Major version bump",
	versioning.BumpMinor:      "Minor version bump",
	versioning.BumpPatch:      "Patch version bump",
	versioning.BumpGraduate:   "Graduate prerelease to stable",
	versioning.BumpPrerelease: "Bump by a prerelease version",
	versioning.BumpCustom:     "A custom version was applied",
}

type VersioningInfo struct {
	ManualBump    bool
	VersionReport *versioning.MergedVersionReport
}

func GetBumpTypeLabels() map[versioning.BumpType]string {
	return bumpTypeLabels
}

func GetLabelBasedVersionBump(pr *github.PullRequest) versioning.BumpType {
	if pr == nil {
		return versioning.BumpNone
	}

	var bumpLabels []versioning.BumpType
	for _, label := range pr.Labels {
		if _, ok := bumpTypeLabels[versioning.BumpType(label.GetName())]; ok {
			bumpLabels = append(bumpLabels, versioning.BumpType(label.GetName()))
		}
	}

	if bumpType := stackRankBumpLabels(bumpLabels); bumpType != versioning.BumpNone {
		currentPRBumpType, currentPRBumpMethod, err := parseBumpFromPRBody(pr.GetBody())
		if err != nil {
			_ = fmt.Errorf("failed to parse bump type and mode from PR body: %w", err)
			return versioning.BumpNone
		}

		// rules for explicit label versioning
		// if the current Bump Type != label based versioning Bump we will use the label based versioning Bump
		// if the current Bump Type == label based versioning Bump  and that was manually set we will stick to it
		if currentPRBumpType != bumpType || currentPRBumpMethod == BumpMethodManual {
			return bumpType
		}
	}

	return versioning.BumpNone
}

func ManualBumpWasUsed(bumpType *versioning.BumpType, versionReport *versioning.MergedVersionReport) bool {
	if bumpType == nil || versionReport == nil {
		return false
	}

	for _, report := range versionReport.Reports {
		if report.BumpType == *bumpType {
			return true
		}
	}

	return false
}

// We get the recorded BumpType and BumpMethod out of the PR body
func parseBumpFromPRBody(prBody string) (versioning.BumpType, BumpMethod, error) {
	// be very careful if changing this regex, it is critical
	re := regexp.MustCompile(`Version Bump Type:\s*\[(\w+)]\s*-\s*(👤|🤖)`)
	matches := re.FindStringSubmatch(prBody)

	// Check if the expected parts were found
	if len(matches) != 3 {
		return "", "", fmt.Errorf("failed to parse bump type and mode from PR body")
	}

	// Extract bump type and mode
	bumpType := matches[1]
	mode := matches[2]
	if _, ok := bumpTypeLabels[versioning.BumpType(bumpType)]; !ok {
		return "", "", fmt.Errorf("invalid bump type: %s", bumpType)
	}

	return versioning.BumpType(bumpType), BumpMethod(mode), nil
}

// If someone happens to have multiple version labels applied we have a specific priority rankings for determining bump type
func stackRankBumpLabels(bumpLabels []versioning.BumpType) versioning.BumpType {
	// Priority order from highest to lowest
	priorityOrder := []versioning.BumpType{
		versioning.BumpGraduate,
		versioning.BumpMajor,
		versioning.BumpMinor,
		versioning.BumpPatch,
	}

	for _, priority := range priorityOrder {
		if slices.Contains(bumpLabels, priority) {
			return priority
		}
	}

	return versioning.BumpNone
}
