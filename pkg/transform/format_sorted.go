package transform

import (
	"bytes"
	stdjson "encoding/json"
	"fmt"
	"io"
	"os"

	openapijson "github.com/speakeasy-api/openapi/json"
	"github.com/speakeasy-api/openapi/sortfmt"
	"gopkg.in/yaml.v3"
)

// FormatSortedDocument formats a JSON or YAML document from a file using deterministic ordering.
func FormatSortedDocument(schemaPath string, yamlOut bool, w io.Writer) error {
	file, err := os.Open(schemaPath)
	if err != nil {
		return err
	}
	defer file.Close()

	return FormatSortedFromReader(file, w, yamlOut)
}

// FormatSortedFromReader formats JSON or YAML from a reader using deterministic ordering.
func FormatSortedFromReader(schema io.Reader, w io.Writer, yamlOut bool) error {
	if yamlOut {
		return fmt.Errorf("sorted formatting only supports JSON output")
	}

	input, err := io.ReadAll(schema)
	if err != nil {
		return fmt.Errorf("read document: %w", err)
	}

	if stdjson.Valid(input) {
		return sortfmt.Format(bytes.NewReader(input), w)
	}

	var document yaml.Node
	if err := yaml.Unmarshal(input, &document); err != nil {
		return fmt.Errorf("parse YAML document: %w", err)
	}

	var jsonInput bytes.Buffer
	if err := openapijson.YAMLToJSON(&document, 2, &jsonInput); err != nil {
		return fmt.Errorf("convert YAML document to JSON: %w", err)
	}

	return sortfmt.Format(&jsonInput, w)
}
