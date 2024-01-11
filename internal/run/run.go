package run

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/download"
	"github.com/speakeasy-api/speakeasy/internal/overlay"
	"github.com/speakeasy-api/speakeasy/internal/sdkgen"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/internal/validation"
	"github.com/speakeasy-api/speakeasy/pkg/merge"
)

func Run(ctx context.Context, target, source, genVersion, installationURL, repo, repoSubDir string, debug bool) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	wf, workflowFileLocation, err := workflow.Load(wd)
	if err != nil {
		return err
	}

	// Get the project directory which is the parent of the .speakeasy folder the workflow file is in
	projectDir := filepath.Dir(filepath.Dir(workflowFileLocation))
	if err := os.Chdir(projectDir); err != nil {
		return err
	}

	if err := wf.Validate(generate.GetSupportedLanguages()); err != nil {
		return err
	}

	targets := []string{}
	for targetID := range wf.Targets {
		targets = append(targets, targetID)
	}
	slices.Sort(targets)

	sources := []string{}
	for sourceID := range wf.Sources {
		sources = append(sources, sourceID)
	}
	slices.Sort(sources)

	if target == "" && source == "" {
		if len(wf.Targets) == 1 {
			target = targets[0]
		} else if len(wf.Targets) == 0 && len(wf.Sources) == 1 {
			source = sources[0]
		} else {
			// TODO update to use our proper interactive code
			prompt := promptui.Prompt{
				Label: fmt.Sprintf("Select a target (%s or 'all')", strings.Join(targets, ", ")),
				Validate: func(input string) error {
					if input == "" {
						return fmt.Errorf("target cannot be empty")
					}

					if input != "all" && !slices.Contains(targets, input) {
						return fmt.Errorf("invalid target")
					}

					return nil
				},
			}

			result, err := prompt.Run()
			if err != nil {
				return err
			}

			target = result
		}
	}

	if source != "" && target != "" {
		return fmt.Errorf("cannot specify both a target and a source")
	}

	if target == "all" {
		for t := range wf.Targets {
			if err := runTarget(ctx, t, wf, projectDir, genVersion, installationURL, repo, repoSubDir, debug); err != nil {
				return err
			}
		}
	} else if source == "all" {
		for id, s := range wf.Sources {
			if _, err := runSource(ctx, id, &s); err != nil {
				return err
			}
		}
	} else if target != "" {
		if _, ok := wf.Targets[target]; !ok {
			return fmt.Errorf("target %s not found", target)
		}

		if err := runTarget(ctx, target, wf, projectDir, genVersion, installationURL, repo, repoSubDir, debug); err != nil {
			return err
		}
	} else if source != "" {
		s, ok := wf.Sources[source]
		if !ok {
			return fmt.Errorf("source %s not found", source)
		}

		if _, err := runSource(ctx, source, &s); err != nil {
			return err
		}
	}

	return nil
}

func runTarget(ctx context.Context, target string, wf *workflow.Workflow, projectDir, genVersion, installationURL, repo, repoSubDir string, debug bool) error {
	t := wf.Targets[target]

	fmt.Printf("Running target %s (%s)...\n", target, t.Target)

	source, sourcePath, err := wf.GetTargetSource(target)
	if err != nil {
		return err
	}

	if source != nil {
		sourcePath, err = runSource(ctx, t.Source, source)
		if err != nil {
			return err
		}
	} else {
		if err := validateDocument(ctx, sourcePath); err != nil {
			return err
		}
	}

	var outDir string
	if t.Output != nil {
		outDir = *t.Output
	} else {
		outDir = projectDir
	}

	published := t.Publishing != nil && t.Publishing.IsPublished(target)

	if err := sdkgen.Generate(ctx, config.GetCustomerID(), config.GetWorkspaceID(), t.Target, sourcePath, "", "", outDir, genVersion, installationURL, debug, true, published, false, repo, repoSubDir, true); err != nil {
		return err
	}

	// Clean up temp files on success
	os.RemoveAll(workflow.GetTempDir())

	return nil
}

func runSource(ctx context.Context, id string, source *workflow.Source) (string, error) {
	fmt.Printf("Running source %s...\n", id)

	outputLocation, err := source.GetOutputLocation()
	if err != nil {
		return "", err
	}

	var currentDocument string

	if len(source.Inputs) == 1 {
		if source.Inputs[0].IsRemote() {
			downloadLocation := outputLocation
			if len(source.Overlays) > 0 {
				downloadLocation = source.Inputs[0].GetTempDownloadPath(workflow.GetTempDir())
			}

			currentDocument, err = resolveRemoteDocument(source.Inputs[0], downloadLocation)
			if err != nil {
				return "", err
			}
		} else {
			currentDocument = source.Inputs[0].Location
		}
	} else {
		mergeLocation := source.GetTempMergeLocation()
		if len(source.Overlays) == 0 {
			mergeLocation = outputLocation
		}

		fmt.Printf("Merging %d schemas into %s...\n", len(source.Inputs), mergeLocation)

		inSchemas := []string{}
		for _, input := range source.Inputs {
			if input.IsRemote() {
				downloadedPath, err := resolveRemoteDocument(input, input.GetTempDownloadPath(workflow.GetTempDir()))
				if err != nil {
					return "", err
				}

				inSchemas = append(inSchemas, downloadedPath)
			} else {
				inSchemas = append(inSchemas, input.Location)
			}
		}

		if err := mergeDocuments(inSchemas, mergeLocation); err != nil {
			return "", err
		}

		currentDocument = mergeLocation
	}

	if len(source.Overlays) > 0 {
		overlayLocation := outputLocation

		fmt.Printf("Apply %d overlays into %s...\n", len(source.Overlays), overlayLocation)

		overlaySchemas := []string{}
		for _, overlay := range source.Overlays {
			if overlay.IsRemote() {
				downloadedPath, err := resolveRemoteDocument(overlay, workflow.GetTempDir())
				if err != nil {
					return "", err
				}

				overlaySchemas = append(overlaySchemas, downloadedPath)
			} else {
				overlaySchemas = append(overlaySchemas, overlay.Location)
			}
		}

		if err := overlayDocument(currentDocument, overlaySchemas, overlayLocation); err != nil {
			return "", err
		}
	}

	if err := validateDocument(ctx, outputLocation); err != nil {
		return "", err
	}

	return outputLocation, nil
}

func resolveRemoteDocument(d workflow.Document, outPath string) (string, error) {
	fmt.Printf("Downloading %s... to %s\n", d.Location, outPath)

	if err := os.MkdirAll(filepath.Dir(outPath), os.ModePerm); err != nil {
		return "", err
	}

	var token, header string
	if d.Auth != nil {
		header = d.Auth.Header
		token = os.Getenv(strings.TrimPrefix(d.Auth.Secret, "$"))
	}

	if err := download.DownloadFile(d.Location, outPath, header, token); err != nil {
		return "", err
	}

	return outPath, nil
}

func mergeDocuments(inSchemas []string, outFile string) error {
	if err := os.MkdirAll(filepath.Dir(outFile), os.ModePerm); err != nil {
		return err
	}

	if err := merge.MergeOpenAPIDocuments(inSchemas, outFile); err != nil {
		return err
	}

	fmt.Println(utils.Green(fmt.Sprintf("Successfully merged %d schemas into %s", len(inSchemas), outFile)))

	return nil
}

func overlayDocument(schema string, overlayFiles []string, outFile string) error {
	currentBase := schema

	if err := os.MkdirAll(filepath.Dir(outFile), os.ModePerm); err != nil {
		return err
	}

	f, err := os.Create(outFile)
	if err != nil {
		return err
	}

	for _, overlayFile := range overlayFiles {
		if err := overlay.Apply(currentBase, overlayFile, f); err != nil {
			return err
		}

		currentBase = outFile
	}

	fmt.Println(utils.Green(fmt.Sprintf("Successfully applied %d overlays into %s", len(overlayFiles), outFile)))

	return nil
}

func validateDocument(ctx context.Context, schemaPath string) error {
	limits := &validation.OutputLimits{
		MaxErrors:   1000,
		MaxWarns:    1000,
		OutputHints: false,
	}

	return validation.ValidateOpenAPI(ctx, schemaPath, "", "", limits)
}
