package usagegen

import (
	"bytes"
	"context"
	"fmt"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/utils"

	"github.com/pkg/errors"
	changelog "github.com/speakeasy-api/openapi-generation/v2"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"go.uber.org/zap"
)

func Generate(ctx context.Context, customerID, lang, schemaPath, header, token, out, operation, namespace, configPath string, all bool, outputBuffer *bytes.Buffer) error {
	matchedLanguage := false
	for _, language := range workflow.SupportedLanguagesUsageSnippets {
		if language == lang {
			matchedLanguage = true
		}
	}

	if !matchedLanguage {
		return fmt.Errorf("language not supported: %s", lang)
	}

	isRemote, schema, err := openapi.GetSchemaContents(ctx, schemaPath, header, token)
	if err != nil {
		return fmt.Errorf("failed to get schema contents: %w", err)
	}

	l := log.From(ctx).WithAssociatedFile(schemaPath)

	tmpOutput := outputBuffer
	if tmpOutput == nil {
		tmpOutput = &bytes.Buffer{}
	}
	opts := []generate.GeneratorOptions{
		generate.WithLogger(l),
		generate.WithCustomerID(customerID),
		generate.WithFileSystem(&fileSystem{buf: tmpOutput}),
		generate.WithRunLocation("cli"),
		generate.WithGenVersion(strings.TrimPrefix(changelog.GetLatestVersion(), "v")),
		generate.WithForceGeneration(),
	}

	if all {
		opts = append(opts, generate.WithUsageSnippetArgsGenerateAll())
	} else if operation == "" && namespace == "" {
		opts = append(opts, generate.WithUsageSnippetArgsByRootExample())
	} else if operation != "" {
		opts = append(opts, generate.WithUsageSnippetArgsByOperationID(operation))
	} else {
		opts = append(opts, generate.WithUsageSnippetArgsByNamespace(namespace))
	}

	g, err := generate.New(opts...)
	if err != nil {
		return err
	}

	if errs := g.Generate(context.Background(), schema, schemaPath, lang, configPath, isRemote, false); len(errs) > 0 {
		for _, err := range errs {
			l.Error("", zap.Error(err))
		}

		return fmt.Errorf("failed to generate usage snippets for %s ✖", lang)
	}

	if out == "" {
		if outputBuffer == nil {
			// By default, write to stdout
			fmt.Println(tmpOutput.String())
		}
	} else if isDirectory(out) {
		return writeFormattedDirectory(lang, out, outputBuffer.String())
	} else {
		file, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			return errors.Wrap(err, "cannot write to provided file")
		}
		defer file.Close()

		_, err = file.WriteString(outputBuffer.String())
		if err != nil {
			return errors.Wrap(err, "cannot write to provided file")
		}
	}

	return nil
}

// writeFormattedDirectory: writes each OperationIDs usage snippet into its own directory with a single main file
// This will be used frequently by things like devcontainers
func writeFormattedDirectory(lang, path, content string) error {
	snippets, err := ParseUsageOutput(lang, content)
	if err != nil {
		return err
	}

	for _, snippet := range snippets {
		// TODO: do we still need to do this?
		//remove the trailing line if it includes an empty comment string
		//if strings.TrimSpace(lines[len(lines)-1]) == "//" || strings.TrimSpace(lines[len(lines)-1]) == "#" {
		//	lines = lines[0 : len(lines)-1]
		//}

		// write out directory structure
		directoryPath := path + "/" + strings.ToLower(snippet.OperationId)
		if err := writeExampleCode(lang, directoryPath, snippet.Snippet); err != nil {
			return err
		}
	}

	return nil
}

type UsageSnippet struct {
	Language    string
	OperationId string
	Method      string
	Path        string
	Snippet     string
}

// Input will look something like:
// Usage snippet provided for ...
// ...
func ParseUsageOutput(lang, s string) ([]UsageSnippet, error) {
	sectionsRegex := regexp.MustCompile(`\s*(?://|#) Usage snippet provided for `)
	sections := sectionsRegex.Split(s, -1)

	snippets := make([]UsageSnippet, len(sections)-1)
	for i, section := range sections[1:] {
		snippet, err := parseOperationInfoAndCodeSample(lang, section)
		if err != nil {
			return nil, err
		}
		snippets[i] = *snippet
	}
	sort.SliceStable(snippets, func(i, j int) bool {
		// First by path, then by method
		if snippets[i].Path != snippets[j].Path {
			return snippets[i].Path < snippets[j].Path
		}
		if snippets[i].Method != snippets[j].Method {
			return snippets[i].Method < snippets[j].Method
		}
		if snippets[i].OperationId != snippets[j].OperationId {
			return snippets[i].OperationId < snippets[j].OperationId
		}
		if snippets[i].Language != snippets[j].Language {
			return snippets[i].Language < snippets[j].Language
		}
		return snippets[i].Snippet < snippets[j].Snippet
	})

	return snippets, nil
}

// Input will look something like:
// getApis (get /v1/apis)
// package main
// ...
func parseOperationInfoAndCodeSample(lang, usageOutputSection string) (*UsageSnippet, error) {
	parts := strings.SplitN(usageOutputSection, "\n", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("expected at least two lines: %s", usageOutputSection)
	}

	// Define a regular expression to capture the API name, method, and endpoint
	apiDetailsRegex := regexp.MustCompile(`([/\w{}_]+)\s+\((\w+)\s+(.*)\)`)

	// Find and extract the API details
	matches := apiDetailsRegex.FindStringSubmatch(parts[0])
	if len(matches) < 3 {
		return nil, fmt.Errorf("failed to extract API details from usage output section: %s", parts[0])
	}

	operationId := matches[1]
	method := matches[2]
	path := matches[3]

	snippet := strings.TrimSpace(parts[1])

	return &UsageSnippet{
		Language:    lang,
		OperationId: operationId,
		Method:      method,
		Path:        path,
		Snippet:     snippet,
	}, nil
}

func writeExampleCode(lang, path, code string) error {
	outFile := ""
	switch lang {
	case "go":
		outFile = path + "/main.go"
	case "csharp":
		outFile = path + "/Program.cs"
	case "unity":
		outFile = path + "/Program.cs"
	case "java":
		outFile = path + "/main.java"
	case "php":
		outFile = path + "/main.php"
	case "python":
		outFile = path + "/main.py"
	case "ruby":
		outFile = path + "/app.rb"
	case "swift":
		outFile = path + "/main.swift"
	case "typescript":
		outFile = path + "/index.ts"
	default:
		return fmt.Errorf("language not supported: %s", lang)
	}

	if err := os.RemoveAll(path); err != nil {
		return err
	}

	if err := utils.CreateDirectory(path); err != nil {
		return err
	}

	if err := os.WriteFile(outFile, []byte(code), 0o644); err != nil {
		return err
	}

	return nil
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.IsDir()
}

type fileSystem struct {
	buf *bytes.Buffer
}

var _ generate.FileSystem = &fileSystem{}

func (fs *fileSystem) ReadFile(fileName string) ([]byte, error) {
	return os.ReadFile(fileName)
}

func (fs *fileSystem) WriteFile(outFileName string, data []byte, mode os.FileMode) error {
	// Make this resilient to additional files being inadvertently written
	if strings.Contains(string(data), "Usage snippet provided") {
		_, err := fs.buf.Write(data)
		return err
	}

	return nil
}

func (fs *fileSystem) MkdirAll(path string, mode os.FileMode) error {
	return nil
}

func (fs *fileSystem) Open(name string) (fs.File, error) {
	return os.Open(name)
}

func (fs *fileSystem) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}
