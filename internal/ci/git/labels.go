package git

import (
	"context"
	"os"
	"strings"

	"github.com/google/go-github/v63/github"
	"github.com/speakeasy-api/speakeasy/internal/ci/logging"
	"github.com/speakeasy-api/speakeasy/internal/ci/versionbumps"
	"github.com/speakeasy-api/versioning-reports/versioning"
)

func (g *Git) UpsertLabelTypes(ctx context.Context) map[string]github.Label {
	desiredLabels := map[string]github.Label{}
	addGitHubLabel := func(name, description string) {
		desiredLabels[name] = github.Label{
			Name:        &name,
			Description: &description,
		}
	}
	for bumpType, description := range versionbumps.GetBumpTypeLabels() {
		addGitHubLabel(string(bumpType), description)
	}

	actualLabels := make(map[string]github.Label)
	allLabels, _, err := g.client.Issues.ListLabels(ctx, os.Getenv("GITHUB_REPOSITORY_OWNER"), GetRepo(), nil)
	if err != nil {
		return actualLabels
	}
	for _, label := range allLabels {
		actualLabels[*label.Name] = *label
	}

	for _, label := range desiredLabels {
		foundLabel, ok := actualLabels[*label.Name]
		if ok {
			if *foundLabel.Description != *label.Description {
				_, _, err = g.client.Issues.EditLabel(ctx, os.Getenv("GITHUB_REPOSITORY_OWNER"), GetRepo(), *label.Name, &github.Label{
					Name:        label.Name,
					Description: label.Description,
				})
				if err != nil {
					return actualLabels
				}
			}
		} else {
			_, _, err = g.client.Issues.CreateLabel(ctx, os.Getenv("GITHUB_REPOSITORY_OWNER"), GetRepo(), &label)
			if err != nil {
				return actualLabels
			}
		}
		actualLabels[*label.Name] = label
	}
	return actualLabels
}

func (g *Git) setPRLabels(background context.Context, owner string, repo string, issueNumber int, labelTypes map[string]github.Label, actualLabels, desiredLabels []*github.Label) {
	shouldRemove := []string{}
	shouldAdd := []string{}
	for _, label := range actualLabels {
		foundInDesired := false
		for _, desired := range desiredLabels {
			if label.GetName() == desired.GetName() {
				foundInDesired = true
				break
			}
			if _, ok := labelTypes[label.GetName()]; !ok {
				foundInDesired = true
				continue
			}
			break
		}

		// We shouldn't delete labels that aren't managed by us
		if _, ok := versionbumps.GetBumpTypeLabels()[versioning.BumpType(label.GetName())]; ok && !foundInDesired {
			shouldRemove = append(shouldRemove, label.GetName())
		}
	}
	for _, desired := range desiredLabels {
		foundInActual := false
		for _, label := range actualLabels {
			if label.GetName() == desired.GetName() {
				foundInActual = true
				break
			}
		}
		if !foundInActual {
			shouldAdd = append(shouldAdd, desired.GetName())
		}
	}
	if len(shouldAdd) > 0 {
		_, _, err := g.client.Issues.AddLabelsToIssue(background, owner, repo, issueNumber, shouldAdd)
		if err != nil {
			logging.Info("failed to add labels %v: %s", shouldAdd, err.Error())
		}
	}
	if len(shouldRemove) > 0 {
		for _, label := range shouldRemove {
			_, err := g.client.Issues.RemoveLabelForIssue(background, owner, repo, issueNumber, label)
			if err != nil {
				logging.Info("failed to remove labels %s: %s", label, err.Error())
			}
		}
	}
}

func PRVersionMetadata(m *versioning.MergedVersionReport, labelTypes map[string]github.Label) (string, *versioning.BumpType, []*github.Label) {
	var labelBumpTypeAdded *versioning.BumpType
	if m == nil {
		return "", labelBumpTypeAdded, []*github.Label{}
	}
	labels := []*github.Label{}
	skipBumpType := false
	skipVersionNumber := false
	singleBumpType := ""
	singleNewVersion := ""
	for _, report := range m.Reports {
		if len(report.BumpType) > 0 && report.BumpType != versioning.BumpNone {
			if len(singleBumpType) > 0 {
				skipBumpType = true
			}
			singleBumpType = string(report.BumpType)
		}
		if len(report.NewVersion) > 0 {
			if len(singleNewVersion) > 0 {
				skipVersionNumber = true
			}
			singleNewVersion = report.NewVersion
		}
	}
	var builder []string
	if !skipVersionNumber {
		builder = append(builder, singleNewVersion)
	}
	if !skipBumpType {
		if matched, ok := labelTypes[singleBumpType]; ok {
			labels = append(labels, &matched)
			bumpType := versioning.BumpType(singleBumpType)
			labelBumpTypeAdded = &bumpType
		}
	}
	// Add an extra " " at front
	if len(builder) > 0 {
		builder = append([]string{""}, builder...)
	}
	return strings.Join(builder, " "), labelBumpTypeAdded, labels
}
