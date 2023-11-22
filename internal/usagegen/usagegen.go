package usagegen

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/schema"

	"github.com/pkg/errors"
	changelog "github.com/speakeasy-api/openapi-generation/v2"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"go.uber.org/zap"
)

var SupportedLanguagesUsageSnippets = []string{
	"go",
	"typescript",
	"python",
	"java",
	"php",
	"swift",
	"ruby",
	"csharp",
	"unity",
}

func Generate(ctx context.Context, customerID, lang, schemaPath, header, token, out, operation, namespace, configPath string) error {
	matchedLanguage := false
	for _, language := range SupportedLanguagesUsageSnippets {
		if language == lang {
			matchedLanguage = true
		}
	}

	if !matchedLanguage {
		return fmt.Errorf("language not supported: %s", lang)
	}

	isRemote, schema, err := schema.GetSchemaContents(schemaPath, header, token)
	if err != nil {
		return fmt.Errorf("failed to get schema contents: %w", err)
	}

	l := log.NewLogger(schemaPath)

	outputBuffer := &bytes.Buffer{}
	opts := []generate.GeneratorOptions{
		generate.WithLogger(l),
		generate.WithCustomerID(customerID),
		generate.WithFileFuncs(writeFileWithBuffer(outputBuffer), os.ReadFile),
		generate.WithRunLocation("cli"),
		generate.WithGenVersion(strings.TrimPrefix(changelog.GetLatestVersion(), "v")),
		generate.WithAllowRemoteReferences(),
	}

	if operation == "" && namespace == "" {
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

	if errs := g.Generate(context.Background(), schema, schemaPath, lang, configPath, isRemote); len(errs) > 0 {
		for _, err := range errs {
			l.Error("", zap.Error(err))
		}

		return fmt.Errorf("failed to generate usage snippets for %s ✖", lang)
	}

	if out == "" {
		// By default, write to stdout
		fmt.Println(outputBuffer.String())
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

func writeFileWithBuffer(buf *bytes.Buffer) func(outFileName string, data []byte, mode os.FileMode) error {
	return func(outFileName string, data []byte, mode os.FileMode) error {
		// Make this resilient to additional files being inadvertently written
		if strings.Contains(string(data), "Usage snippet provided for") {
			_, err := buf.Write(data)
			return err
		}

		return nil
	}
}

// writeFormattedDirectory: writes each OperationIDs usage snippet into its own directory with a single main file
// This will be used frequently by things like devcontainers
func writeFormattedDirectory(lang, path, content string) error {
	// Split the input string by the key phrase "Usage snippet provided for"
	snippets := strings.Split(content, "Usage snippet provided for")

	// Throw out the first line it will just be // or #
	if strings.Contains(snippets[0], "//") || strings.Contains(snippets[0], "#") {
		snippets = snippets[1:]
	}

	for _, snippet := range snippets {
		lines := strings.Split(snippet, "\n")

		// grab the operation name and trim it out of the snippet
		operationName := strings.TrimSpace(lines[0])
		lines = lines[1:]

		// remove the trailing line if it includes an empty comment string
		if strings.TrimSpace(lines[len(lines)-1]) == "//" || strings.TrimSpace(lines[len(lines)-1]) == "#" {
			lines = lines[0 : len(lines)-1]
		}

		// trim empty lines at the end of the snippet
		for len(strings.TrimSpace(lines[len(lines)-1])) == 0 {
			lines = lines[0 : len(lines)-1]
		}

		// write out directory structure
		directoryPath := path + "/" + strings.ToLower(operationName)
		if err := writeExampleCode(lang, directoryPath, strings.Join(lines, "\n")); err != nil {
			return err
		}
	}

	return nil
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

	if err := os.MkdirAll(path, 0o755); err != nil {
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
