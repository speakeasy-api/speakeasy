package github_test

import (
	"fmt"
	"testing"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/speakeasy/internal/github"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestSortErrors(t *testing.T) {
	t.Parallel()

	type args struct {
		errs []error
	}
	tests := []struct {
		name string
		args args
		want []error
	}{
		{
			name: "Sort errors by severity and line number",
			args: args{
				errs: []error{
					fmt.Errorf("random error"),
					errors.NewValidationError("error 2", &yaml.Node{Line: 10}, fmt.Errorf("validation error 2")),
					errors.NewValidationWarning("warning 1", &yaml.Node{Line: 2}, fmt.Errorf("validation warning 1")),
					errors.NewValidationWarning("warning 2", &yaml.Node{Line: 3}, fmt.Errorf("validation warning 2")),
					errors.NewUnsupportedError("unsupported 2", &yaml.Node{Line: 5}),
					errors.NewUnsupportedError("unsupported 1", &yaml.Node{Line: 4}),
					&errors.ValidationError{
						Severity: errors.SeverityHint,
						Message:  "hint 2",
						Node:     &yaml.Node{Line: 7},
						Cause:    fmt.Errorf("validation hint 2"),
					},
					&errors.ValidationError{
						Severity: errors.SeverityHint,
						Message:  "hint 1",
						Node:     &yaml.Node{Line: 6},
						Cause:    fmt.Errorf("validation hint 1"),
					},
					errors.NewValidationError("error 1", &yaml.Node{Line: 1}, fmt.Errorf("validation error 1")),
				},
			},
			want: []error{
				errors.NewValidationError("error 1", &yaml.Node{Line: 1}, fmt.Errorf("validation error 1")),
				errors.NewValidationError("error 2", &yaml.Node{Line: 10}, fmt.Errorf("validation error 2")),
				errors.NewValidationWarning("warning 1", &yaml.Node{Line: 2}, fmt.Errorf("validation warning 1")),
				errors.NewValidationWarning("warning 2", &yaml.Node{Line: 3}, fmt.Errorf("validation warning 2")),
				&errors.ValidationError{
					Severity: errors.SeverityHint,
					Message:  "hint 1",
					Node:     &yaml.Node{Line: 6},
					Cause:    fmt.Errorf("validation hint 1"),
				},
				&errors.ValidationError{
					Severity: errors.SeverityHint,
					Message:  "hint 2",
					Node:     &yaml.Node{Line: 7},
					Cause:    fmt.Errorf("validation hint 2"),
				},
				errors.NewUnsupportedError("unsupported 1", &yaml.Node{Line: 4}),
				errors.NewUnsupportedError("unsupported 2", &yaml.Node{Line: 5}),
				fmt.Errorf("random error"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			github.SortErrors(tt.args.errs)

			assert.Equal(t, tt.want, tt.args.errs)
		})
	}
}
