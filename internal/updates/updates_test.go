package updates

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v63/github"
	"github.com/hashicorp/go-version"
)

func TestDirectAssetURL(t *testing.T) {
	tests := []struct {
		name         string
		version      string
		artifactArch string
		want         string
	}{
		{
			name:         "linux amd64",
			version:      "1.773.1",
			artifactArch: "linux_amd64",
			want:         "https://github.com/speakeasy-api/speakeasy/releases/download/v1.773.1/speakeasy_linux_amd64.zip",
		},
		{
			name:         "darwin arm64",
			version:      "1.256.0",
			artifactArch: "darwin_arm64",
			want:         "https://github.com/speakeasy-api/speakeasy/releases/download/v1.256.0/speakeasy_darwin_arm64.zip",
		},
		{
			name:         "arch is lowercased",
			version:      "1.500.0",
			artifactArch: "Windows_AMD64",
			want:         "https://github.com/speakeasy-api/speakeasy/releases/download/v1.500.0/speakeasy_windows_amd64.zip",
		},
		{
			name:         "v prefix in input is normalized",
			version:      "v1.773.1",
			artifactArch: "linux_amd64",
			want:         "https://github.com/speakeasy-api/speakeasy/releases/download/v1.773.1/speakeasy_linux_amd64.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := version.NewVersion(tt.version)
			if err != nil {
				t.Fatalf("version.NewVersion(%q): %v", tt.version, err)
			}
			if got := directAssetURL(v, tt.artifactArch); got != tt.want {
				t.Errorf("directAssetURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDescribeGitHubError(t *testing.T) {
	t.Run("rate limit error mentions rate limiting and GITHUB_TOKEN", func(t *testing.T) {
		rateLimitErr := &github.RateLimitError{
			Rate: github.Rate{
				Limit:     60,
				Remaining: 0,
				Reset:     github.Timestamp{Time: time.Now().Add(30 * time.Minute)},
			},
			Message: "API rate limit exceeded",
		}
		got := describeGitHubError(rateLimitErr)
		if !strings.Contains(got.Error(), "rate limit") {
			t.Errorf("expected error to mention rate limiting, got: %v", got)
		}
		if !strings.Contains(got.Error(), "GITHUB_TOKEN") {
			t.Errorf("expected error to mention GITHUB_TOKEN, got: %v", got)
		}
		if !errors.As(got, new(*github.RateLimitError)) {
			t.Errorf("expected original error to remain unwrappable, got: %v", got)
		}
	})

	t.Run("wrapped rate limit error is still detected", func(t *testing.T) {
		wrapped := fmt.Errorf("outer: %w", &github.RateLimitError{})
		if !strings.Contains(describeGitHubError(wrapped).Error(), "rate limit") {
			t.Errorf("expected wrapped rate limit error to be detected")
		}
	})

	t.Run("abuse rate limit error mentions GITHUB_TOKEN", func(t *testing.T) {
		got := describeGitHubError(&github.AbuseRateLimitError{Message: "abuse"})
		if !strings.Contains(got.Error(), "GITHUB_TOKEN") {
			t.Errorf("expected error to mention GITHUB_TOKEN, got: %v", got)
		}
	})

	t.Run("other errors pass through unchanged", func(t *testing.T) {
		orig := errors.New("connection refused")
		if got := describeGitHubError(orig); got != orig {
			t.Errorf("expected passthrough, got: %v", got)
		}
	})
}

func TestGithubClientTokenSelection(t *testing.T) {
	t.Run("no token env vars yields unauthenticated client", func(t *testing.T) {
		for _, key := range []string{"SPEAKEASY_GITHUB_TOKEN", "GITHUB_TOKEN", "GH_TOKEN"} {
			t.Setenv(key, "")
		}
		client := githubClient(time.Second)
		if _, ok := client.Client().Transport.(*http.Transport); client.Client().Transport != nil && !ok {
			t.Errorf("expected default transport when no token is set, got %T", client.Client().Transport)
		}
	})

	t.Run("SPEAKEASY_GITHUB_TOKEN takes precedence", func(t *testing.T) {
		t.Setenv("SPEAKEASY_GITHUB_TOKEN", "speakeasy-token")
		t.Setenv("GITHUB_TOKEN", "github-token")
		client := githubClient(time.Second)
		if client.Client().Transport == nil {
			t.Fatalf("expected auth transport to be installed when token env var is set")
		}
	})

	t.Run("GH_TOKEN is honored when others are unset", func(t *testing.T) {
		for _, key := range []string{"SPEAKEASY_GITHUB_TOKEN", "GITHUB_TOKEN"} {
			t.Setenv(key, "")
		}
		t.Setenv("GH_TOKEN", "gh-token")
		client := githubClient(time.Second)
		if client.Client().Transport == nil {
			t.Fatalf("expected auth transport to be installed when GH_TOKEN is set")
		}
	})
}
