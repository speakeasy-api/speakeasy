package schema

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/manifoldco/promptui"
	"github.com/speakeasy-api/speakeasy/internal/download"
)

var outputFilePath = "openapi"

func GetSchemaContents(schemaPath string, header, token string) (bool, []byte, error) {
	if _, err := os.Stat(schemaPath); err == nil {
		schema, err := os.ReadFile(schemaPath)
		if err != nil {
			return false, nil, fmt.Errorf("failed to read schema file %s: %w", schemaPath, err)
		}
		return false, schema, nil
	} else {
		u, err := url.Parse(schemaPath)
		if err != nil {
			return false, nil, fmt.Errorf("failed to parse schema url: %w", err)
		}

		if extension := filepath.Ext(u.Path); extension != "" {
			outputFilePath = outputFilePath + extension
		}

		defer func() {
			if err := os.Remove(outputFilePath); err != nil {
				fmt.Println(promptui.Styler(promptui.FGRed, promptui.FGBold)(fmt.Sprintf("failed to delete downloaded schema file: %s", err.Error())))
			}
		}()

		if err := download.DownloadFile(u.String(), outputFilePath, header, token); err != nil {
			return false, nil, fmt.Errorf("failed to download OpenAPI schema: %w", err)
		}

		schema, err := os.ReadFile(outputFilePath)
		if err != nil {
			return false, nil, fmt.Errorf("failed to read schema file %s: %w", schemaPath, err)
		}
		return true, schema, nil
	}
}
