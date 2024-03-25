package changes

import (
	"bytes"
	changes "github.com/speakeasy-api/openapi-changes/cmd"
	"io"
	"strings"
)

type VersionBump string

var (
	Major VersionBump = "major"
	Minor VersionBump = "minor"
	Patch VersionBump = "patch"
	None  VersionBump = "none"
)

type ChangesSummary struct {
	Bump    VersionBump
	Summary string
}

func GetSummary(left, right string) (ChangesSummary, error) {
	bump := None
	out := &bytes.Buffer{}

	breaking, err := runSummaryInternal(left, right, out)
	if err != nil {
		return ChangesSummary{}, err
	}

	summary := out.String()
	if breaking {
		bump = Major
	} else if strings.Contains(summary, "Additions: ") {
		bump = Minor
	} else if strings.Contains(summary, "Modifications: ") {
		bump = Patch
	}

	return ChangesSummary{
		Bump:    bump,
		Summary: summary,
	}, nil
}

// RunSummary runs the summary command and prints the output to the terminal
// The purpose of this utility is that it handles the "breaking changes discovered" error.
// Instead, it only errors when there is a real error.
func RunSummary(left, right string) error {
	_, err := runSummaryInternal(left, right, nil)
	return err
}

// First return value is whether there are breaking changes
func runSummaryInternal(left, right string, outOverride io.Writer) (bool, error) {
	cmd := changes.GetSummaryCommand()

	if outOverride != nil {
		cmd.SetOut(outOverride)
	}

	if err := cmd.RunE(cmd, []string{left, right}); err != nil {
		if strings.Contains(err.Error(), "breaking changes discovered") {
			return true, nil
		} else {
			return false, err
		}
	}

	return false, nil
}
