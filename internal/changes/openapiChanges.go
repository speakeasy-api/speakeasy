package changes

import (
	"errors"
	changesModel "github.com/pb33f/openapi-changes/model"
	"github.com/speakeasy-api/speakeasy-core/changes"
	html_report "github.com/speakeasy-api/speakeasy-core/changes/html-report"
	"os"
	"strings"
	"time"
)

type Changes []*changesModel.Commit
type VersionBump string

type Summary struct {
	Bump  VersionBump
	Text  string
	Table [][]string
}

var (
	Major VersionBump = "major"
	Minor VersionBump = "minor"
	Patch VersionBump = "patch"
	None  VersionBump = "none"
)

func GetChanges(oldLocation, newLocation string) (Changes, error) {
	c, errs := changes.GetChanges(oldLocation, newLocation, changes.SummaryOptions{})
	return c, errors.Join(errs...)
}

func (c Changes) GetHTMLReport() []byte {
	generator := html_report.NewHTMLReport(false, time.Now(), c)
	return generator.GenerateReport(false, false, false)
}

func (c Changes) WriteHTMLReport(out string) error {
	return os.WriteFile(out, c.GetHTMLReport(), 0o644)
}

func (c Changes) GetSummary() (*Summary, error) {
	text, table, hasBreakingChanges, err := changes.GetSummaryDetails(c)
	if err != nil {
		return nil, err
	}

	bump := None

	if hasBreakingChanges {
		bump = Major
	} else if strings.Contains(text, "Additions: ") {
		bump = Minor
	} else if strings.Contains(text, "Modifications: ") {
		bump = Patch
	}

	return &Summary{
		Bump:  bump,
		Text:  text,
		Table: table,
	}, nil
}
