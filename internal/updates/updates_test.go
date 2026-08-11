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
	t.Parallel()
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
			t.Parallel()
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

// describeGitHubError reads the token env vars, so subtests control them with
// t.Setenv, which is incompatible with t.Parallel (including on this parent).
//
//nolint:paralleltest
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

	t.Run("authenticated rate limit reports actual limit without suggesting GITHUB_TOKEN", func(t *testing.T) {
		rateLimitErr := &github.RateLimitError{
			Rate: github.Rate{
				Limit:     5000,
				Remaining: 0,
				Reset:     github.Timestamp{Time: time.Now().Add(30 * time.Minute)},
			},
			Message: "API rate limit exceeded",
		}
		got := describeGitHubError(rateLimitErr)
		if !strings.Contains(got.Error(), "5000 requests/hour") {
			t.Errorf("expected error to report the actual limit, got: %v", got)
		}
		if strings.Contains(got.Error(), "GITHUB_TOKEN") {
			t.Errorf("expected no GITHUB_TOKEN suggestion when already authenticated, got: %v", got)
		}
	})

	t.Run("wrapped rate limit error is still detected", func(t *testing.T) {
		wrapped := fmt.Errorf("outer: %w", &github.RateLimitError{})
		if !strings.Contains(describeGitHubError(wrapped).Error(), "rate limit") {
			t.Errorf("expected wrapped rate limit error to be detected")
		}
	})

	t.Run("abuse rate limit error mentions GITHUB_TOKEN", func(t *testing.T) {
		clearGitHubTokenEnv(t)
		got := describeGitHubError(&github.AbuseRateLimitError{Message: "abuse"})
		if !strings.Contains(got.Error(), "GITHUB_TOKEN") {
			t.Errorf("expected error to mention GITHUB_TOKEN, got: %v", got)
		}
	})

	t.Run("authenticated abuse rate limit reports retry delay without suggesting GITHUB_TOKEN", func(t *testing.T) {
		clearGitHubTokenEnv(t)
		t.Setenv("GITHUB_TOKEN", "github-token")
		retryAfter := 2 * time.Minute
		got := describeGitHubError(&github.AbuseRateLimitError{Message: "abuse", RetryAfter: &retryAfter})
		if !strings.Contains(got.Error(), "retry after 2m0s") {
			t.Errorf("expected error to report the retry delay, got: %v", got)
		}
		if strings.Contains(got.Error(), "GITHUB_TOKEN") {
			t.Errorf("expected no GITHUB_TOKEN suggestion when already authenticated, got: %v", got)
		}
	})

	t.Run("other errors pass through unchanged", func(t *testing.T) {
		orig := errors.New("connection refused")
		//nolint:errorlint // identity comparison is the point: the error must pass through unwrapped
		if got := describeGitHubError(orig); got != orig {
			t.Errorf("expected passthrough, got: %v", got)
		}
	})
}

// clearGitHubTokenEnv unsets every token env var githubToken consults, so a
// token in the test environment can't leak into token-sensitive assertions.
func clearGitHubTokenEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"SPEAKEASY_GITHUB_TOKEN", "GITHUB_TOKEN", "GH_TOKEN"} {
		t.Setenv(key, "")
	}
}

// recordingTransport captures the Authorization header of each request and
// returns a canned response without touching the network.
type recordingTransport struct {
	lastAuthHeader string
}

func (rt *recordingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	rt.lastAuthHeader = r.Header.Get("Authorization")
	return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody, Request: r}, nil
}

func TestGithubClientTokenSelection(t *testing.T) {
	// githubClient builds its client with a nil Transport, so WithAuthToken
	// wraps http.DefaultTransport. Swap in a recorder to observe which token
	// (if any) is actually sent.
	recorder := &recordingTransport{}
	original := http.DefaultTransport
	http.DefaultTransport = recorder
	t.Cleanup(func() { http.DefaultTransport = original })

	sentAuthHeader := func(t *testing.T) string {
		t.Helper()
		client := githubClient(time.Second)
		resp, err := client.Client().Get("https://api.github.com/rate_limit")
		if err != nil {
			t.Fatalf("stubbed request failed: %v", err)
		}
		resp.Body.Close()
		return recorder.lastAuthHeader
	}

	//nolint:paralleltest // clearGitHubTokenEnv uses t.Setenv, which is incompatible with t.Parallel
	t.Run("no token env vars yields unauthenticated requests", func(t *testing.T) {
		clearGitHubTokenEnv(t)
		if got := sentAuthHeader(t); got != "" {
			t.Errorf("expected no Authorization header, got %q", got)
		}
	})

	t.Run("SPEAKEASY_GITHUB_TOKEN takes precedence over GITHUB_TOKEN and GH_TOKEN", func(t *testing.T) {
		clearGitHubTokenEnv(t)
		t.Setenv("SPEAKEASY_GITHUB_TOKEN", "speakeasy-token")
		t.Setenv("GITHUB_TOKEN", "github-token")
		t.Setenv("GH_TOKEN", "gh-token")
		if got := sentAuthHeader(t); got != "Bearer speakeasy-token" {
			t.Errorf("expected SPEAKEASY_GITHUB_TOKEN to win, got Authorization %q", got)
		}
	})

	t.Run("GITHUB_TOKEN takes precedence over GH_TOKEN", func(t *testing.T) {
		clearGitHubTokenEnv(t)
		t.Setenv("GITHUB_TOKEN", "github-token")
		t.Setenv("GH_TOKEN", "gh-token")
		if got := sentAuthHeader(t); got != "Bearer github-token" {
			t.Errorf("expected GITHUB_TOKEN to win, got Authorization %q", got)
		}
	})

	t.Run("GH_TOKEN is honored when others are unset", func(t *testing.T) {
		clearGitHubTokenEnv(t)
		t.Setenv("GH_TOKEN", "gh-token")
		if got := sentAuthHeader(t); got != "Bearer gh-token" {
			t.Errorf("expected GH_TOKEN to be used, got Authorization %q", got)
		}
	})
}
