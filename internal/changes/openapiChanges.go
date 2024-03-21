package changes

import (
	"bytes"
	changes "github.com/speakeasy-api/openapi-changes/cmd"
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
	cmd := changes.GetSummaryCommand()

	bump := None

	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := cmd.RunE(cmd, []string{left, right}); err != nil {
		if strings.Contains(err.Error(), "breaking changes discovered") {
			bump = Major
		} else {
			return ChangesSummary{}, err
		}
	}

	summary := out.String()
	// Breaking changes return an error, so everything at this point is a minor or patch bump
	if strings.Contains(summary, "Additions: ") && bump == None {
		bump = Minor
	} else if strings.Contains(summary, "Modifications: ") && bump == None {
		bump = Patch
	}

	return ChangesSummary{
		Bump:    bump,
		Summary: summary,
	}, nil
}
