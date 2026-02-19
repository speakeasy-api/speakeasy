package updates

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v58/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testTimeout = 30 * time.Second

func skipUnlessIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("SPEAKEASY_INTEGRATION") == "" {
		t.Skip("skipping integration test; set SPEAKEASY_INTEGRATION=1 to run")
	}
}

// TestFallbackList verifies that the fallback caching proxy returns a
// non-empty list of releases that can be decoded as GitHub RepositoryRelease objects.
func TestFallbackList(t *testing.T) {
	t.Parallel()
	skipUnlessIntegration(t)

	releases, err := fetchReleasesFromFallback(testTimeout)
	require.NoError(t, err, "fetchReleasesFromFallback should not error")
	require.NotEmpty(t, releases, "should return at least one release")

	// Spot-check the first release has basic fields populated.
	first := releases[0]
	assert.NotEmpty(t, first.GetTagName(), "first release should have a tag name")
	assert.True(t, strings.HasPrefix(first.GetTagName(), "v"), "tag should start with 'v'")
	assert.NotEmpty(t, first.Assets, "first release should have assets")

	// Verify at least one asset has a download URL.
	var hasDownloadURL bool
	for _, asset := range first.Assets {
		if asset.GetBrowserDownloadURL() != "" {
			hasDownloadURL = true
			break
		}
	}
	assert.True(t, hasDownloadURL, "at least one asset should have a browser download URL")
}

// TestFallbackDownload verifies that the fallback download endpoint returns a
// signed URL for a known release asset.
func TestFallbackDownload(t *testing.T) {
	t.Parallel()
	skipUnlessIntegration(t)

	// First, get a real tag and asset name from the list endpoint.
	releases, err := fetchReleasesFromFallback(testTimeout)
	require.NoError(t, err)
	require.NotEmpty(t, releases)

	var tag, asset string
	for _, r := range releases {
		for _, a := range r.Assets {
			name := a.GetName()
			if strings.Contains(strings.ToLower(name), "linux_amd64") && strings.HasSuffix(name, ".zip") {
				tag = r.GetTagName()
				asset = name
				break
			}
		}
		if tag != "" {
			break
		}
	}
	require.NotEmpty(t, tag, "should find a linux_amd64 asset in releases")
	require.NotEmpty(t, asset)

	// Build a GitHub-style download URL to pass to getFallbackDownloadURL.
	link := "https://github.com/speakeasy-api/speakeasy/releases/download/" + tag + "/" + asset
	signedURL, err := getFallbackDownloadURL(link, testTimeout)
	require.NoError(t, err, "getFallbackDownloadURL should not error")
	require.NotEmpty(t, signedURL, "should return a non-empty signed URL")

	// The signed URL should be a valid URL pointing to GCS.
	parsed, err := url.Parse(signedURL)
	require.NoError(t, err, "signed URL should be parseable")
	assert.Equal(t, "https", parsed.Scheme)
	assert.Contains(t, parsed.Host, "storage.googleapis.com")

	// HEAD the signed URL to verify it's reachable.
	c := &http.Client{Timeout: testTimeout}
	resp, err := c.Head(signedURL)
	require.NoError(t, err, "HEAD on signed URL should not error")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "signed URL should return 200")
}

// TestFallbackListRawHTTP exercises the raw HTTP endpoint directly (no helper)
// to ensure the JSON shape matches what the Go GitHub client expects.
func TestFallbackListRawHTTP(t *testing.T) {
	t.Parallel()
	skipUnlessIntegration(t)

	c := &http.Client{Timeout: testTimeout}
	resp, err := c.Get(fallbackBaseURL + "?action=list")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var releases []*github.RepositoryRelease
	err = json.NewDecoder(resp.Body).Decode(&releases)
	require.NoError(t, err, "response should decode into []*github.RepositoryRelease")
	require.NotEmpty(t, releases)
}
