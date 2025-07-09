package prompts

import (
	"testing"
)

func TestCheckTerraformRepositoryNaming(t *testing.T) {
	tests := []struct {
		name     string
		repoName string
		wantErr  bool
	}{
		{
			name:     "Valid terraform provider name",
			repoName: "terraform-provider-aws",
			wantErr:  false,
		},
		{
			name:     "Valid terraform provider name with numbers",
			repoName: "terraform-provider-aws-v2",
			wantErr:  false,
		},
		{
			name:     "Valid terraform provider name with hyphens",
			repoName: "terraform-provider-google-cloud",
			wantErr:  false,
		},
		{
			name:     "Invalid - missing terraform-provider prefix",
			repoName: "aws-provider",
			wantErr:  true,
		},
		{
			name:     "Invalid - uppercase letters",
			repoName: "terraform-provider-AWS",
			wantErr:  true,
		},
		{
			name:     "Invalid - starts with uppercase",
			repoName: "Terraform-provider-aws",
			wantErr:  true,
		},
		{
			name:     "Invalid - empty name after prefix",
			repoName: "terraform-provider-",
			wantErr:  true,
		},
		{
			name:     "Invalid - just terraform-provider",
			repoName: "terraform-provider",
			wantErr:  true,
		},
		{
			name:     "Invalid - special characters",
			repoName: "terraform-provider-aws@v2",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkTerraformRepositoryNaming(tt.repoName)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkTerraformRepositoryNaming() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if !IsTerraformNamingWarning(err) {
					t.Errorf("Expected TerraformNamingWarning, got %T", err)
				}
			}
		})
	}
}

func TestIsTerraformNamingWarning(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "TerraformNamingWarning",
			err:  &TerraformNamingWarning{RepoName: "test"},
			want: true,
		},
		{
			name: "Other error",
			err:  &TerraformNamingWarning{RepoName: "test"},
			want: true,
		},
		{
			name: "Nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTerraformNamingWarning(tt.err); got != tt.want {
				t.Errorf("IsTerraformNamingWarning() = %v, want %v", got, tt.want)
			}
		})
	}
}
