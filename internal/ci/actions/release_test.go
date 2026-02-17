package actions

import (
	"testing"

	"github.com/speakeasy-api/speakeasy/internal/ci/environment"
)

// TestBranchNameSanitizationForOCITags verifies that branch names are properly
// sanitized for use as OCI registry tags. OCI/Docker tags cannot contain forward
// slashes, so branches like "releases/2025.01" must be converted to "releases-2025.01".
// This test documents the expected behavior used in addCurrentBranchTagging.
func TestBranchNameSanitizationForOCITags(t *testing.T) {
	tests := []struct {
		name         string
		branchName   string
		expectedTag  string
	}{
		{
			name:        "simple branch",
			branchName:  "main",
			expectedTag: "main",
		},
		{
			name:        "versioned release branch with slash",
			branchName:  "releases/2025.01",
			expectedTag: "releases-2025.01",
		},
		{
			name:        "versioned branch with slash",
			branchName:  "versions/2025.10",
			expectedTag: "versions-2025.10",
		},
		{
			name:        "feature branch with slash",
			branchName:  "feature/add-auth",
			expectedTag: "feature-add-auth",
		},
		{
			name:        "branch with multiple slashes",
			branchName:  "releases/2025.01/hotfix",
			expectedTag: "releases-2025.01-hotfix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This mirrors the logic in addCurrentBranchTagging:
			// tags := []string{environment.SanitizeBranchName(branch)}
			tag := environment.SanitizeBranchName(tt.branchName)
			if tag != tt.expectedTag {
				t.Errorf("SanitizeBranchName(%q) = %q, want %q", tt.branchName, tag, tt.expectedTag)
			}
		})
	}
}

func TestGetDirAndShouldUseReleasesMD(t *testing.T) {
	type args struct {
		files           []string
		dir             string
		usingReleasesMd bool
	}
	tests := []struct {
		name  string
		args  args
		want  string
		want1 bool
	}{
		{
			name: "RELEASES.md found",
			args: args{
				files:           []string{"./RELEASES.md", "some/other/file.go"},
				dir:             ".",
				usingReleasesMd: false,
			},
			want:  ".",
			want1: true,
		},
		{
			name: "RELEASES.md found in subdirectory",
			args: args{
				files:           []string{"subdir/RELEASES.md", "some/other/file.go"},
				dir:             ".",
				usingReleasesMd: false,
			},
			want:  "subdir",
			want1: true,
		},
		{
			name: "gen.lock found",
			args: args{
				files:           []string{".speakeasy/gen.lock", "some/other/file.go"},
				dir:             ".",
				usingReleasesMd: false,
			},
			want:  ".",
			want1: false,
		},
		{
			name: "gen.lock found in subdirectory",
			args: args{
				files:           []string{"subdir/.speakeasy/gen.lock", "some/other/file.go"},
				dir:             ".",
				usingReleasesMd: false,
			},
			want:  "subdir",
			want1: false,
		},
		{
			name: "no relevant files found",
			args: args{
				files:           []string{"some/file.go", "another/file.js"},
				dir:             ".",
				usingReleasesMd: false,
			},
			want:  ".",
			want1: false,
		},
		{
			name: "gen.lock takes precedence over RELEASES.md",
			args: args{
				files:           []string{".speakeasy/gen.lock", "RELEASES.md"},
				dir:             ".",
				usingReleasesMd: false,
			},
			want:  ".",
			want1: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := GetDirAndShouldUseReleasesMD(tt.args.files, tt.args.dir, tt.args.usingReleasesMd)
			if got != tt.want {
				t.Errorf("GetDirAndShouldUseReleasesMD() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("GetDirAndShouldUseReleasesMD() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
