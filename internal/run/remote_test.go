package run

import (
	"testing"
)

func TestParseGitHubRepoURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantOrg  string
		wantRepo string
		wantErr  bool
	}{
		{
			name:     "HTTPS URL",
			url:      "https://github.com/speakeasy-api/speakeasy-cli",
			wantOrg:  "speakeasy-api",
			wantRepo: "speakeasy-cli",
			wantErr:  false,
		},
		{
			name:     "HTTPS URL with trailing slash",
			url:      "https://github.com/speakeasy-api/speakeasy-cli/",
			wantOrg:  "speakeasy-api",
			wantRepo: "speakeasy-cli",
			wantErr:  false,
		},
		{
			name:     "SSH URL",
			url:      "git@github.com:speakeasy-api/speakeasy-cli.git",
			wantOrg:  "speakeasy-api",
			wantRepo: "speakeasy-cli",
			wantErr:  false,
		},
		{
			name:     "Simple org/repo format",
			url:      "speakeasy-api/speakeasy-cli",
			wantOrg:  "speakeasy-api",
			wantRepo: "speakeasy-cli",
			wantErr:  false,
		},
		{
			name:    "Invalid URL format",
			url:     "github.com/speakeasy-api/speakeasy-cli",
			wantErr: true,
		},
		{
			name:    "Invalid org/repo format",
			url:     "speakeasy-api-speakeasy-cli",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOrg, gotRepo, err := parseGitHubRepoURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseGitHubRepoURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotOrg != tt.wantOrg {
				t.Errorf("parseGitHubRepoURL() gotOrg = %v, want %v", gotOrg, tt.wantOrg)
			}
			if gotRepo != tt.wantRepo {
				t.Errorf("parseGitHubRepoURL() gotRepo = %v, want %v", gotRepo, tt.wantRepo)
			}
		})
	}
}
