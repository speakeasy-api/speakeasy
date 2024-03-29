package schema

import (
	"context"
	"fmt"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"go.uber.org/zap"
	"net/url"
	"os"
	"path/filepath"

	"github.com/speakeasy-api/speakeasy/internal/download"
)

var outputFilePath = "openapi"

func GetSchemaContents(ctx context.Context, schemaPath string, header, token string) (bool, []byte, error) {
	if _, err := os.Stat(schemaPath); err == nil {
		schema, err := os.ReadFile(schemaPath)
		if err != nil {
			return false, nil, fmt.Errorf("failed to read schema file %s: %w", schemaPath, err)
		}
		return false, schema, nil
	} else {
		println(err.Error())
		u, err := url.Parse(schemaPath)
		if err != nil {
			return false, nil, fmt.Errorf("failed to parse schema url: %w", err)
		}

		if extension := filepath.Ext(u.Path); extension != "" {
			outputFilePath = outputFilePath + extension
		}

		defer func() {
			if err := os.Remove(outputFilePath); err != nil {
				log.From(ctx).Error("failed to delete downloaded schema file", zap.Error(err))
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
