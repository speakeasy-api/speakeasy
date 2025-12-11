package schemas

import (
	"context"
	"testing"
)

func TestIsSwaggerDocument(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
		wantErr  bool
	}{
		{
			name:     "Swagger 2.0 YAML file",
			path:     "../../integration/resources/swagger.yaml",
			expected: true,
			wantErr:  false,
		},
		{
			name:     "OpenAPI 3.0 YAML file",
			path:     "../../integration/resources/converted.yaml",
			expected: false,
			wantErr:  false,
		},
		{
			name:     "Non-existent file",
			path:     "../../integration/resources/nonexistent.yaml",
			expected: false,
			wantErr:  true,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsSwaggerDocument(ctx, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsSwaggerDocument() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("IsSwaggerDocument() = %v, expected %v", got, tt.expected)
			}
		})
	}
}
