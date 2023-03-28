package github_test

import (
	"fmt"
	"testing"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/speakeasy/internal/github"
	"github.com/stretchr/testify/assert"
)

func TestSortErrors(t *testing.T) {
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
					errors.NewValidationError("error 2", 10, fmt.Errorf("validation error 2")),
					errors.NewValidationWarning("warning 1", 2, fmt.Errorf("validation warning 1")),
					errors.NewValidationWarning("warning 2", 3, fmt.Errorf("validation warning 2")),
					errors.NewUnsupportedError("unsupported 2", 5),
					errors.NewUnsupportedError("unsupported 1", 4),
					errors.NewValidationError("error 1", 1, fmt.Errorf("validation error 1")),
				},
			},
			want: []error{
				errors.NewValidationError("error 1", 1, fmt.Errorf("validation error 1")),
				errors.NewValidationError("error 2", 10, fmt.Errorf("validation error 2")),
				errors.NewValidationWarning("warning 1", 2, fmt.Errorf("validation warning 1")),
				errors.NewValidationWarning("warning 2", 3, fmt.Errorf("validation warning 2")),
				errors.NewUnsupportedError("unsupported 1", 4),
				errors.NewUnsupportedError("unsupported 2", 5),
				fmt.Errorf("random error"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			github.SortErrors(tt.args.errs)

			assert.Equal(t, tt.want, tt.args.errs)
		})
	}
}
