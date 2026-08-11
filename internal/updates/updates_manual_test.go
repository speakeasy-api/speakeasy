//go:build manual

package updates

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/speakeasy-api/speakeasy-core/events"
)

// These tests exercise InstallVersion end-to-end against real GitHub
// infrastructure and are gated behind the "manual" build tag:
//
//	go test -tags manual ./internal/updates/ -run TestManual -v
//
// They simulate a rate-limited/unreachable GitHub API by replacing
// http.DefaultTransport (which all clients in this package use, having no
// explicit Transport) with one that rejects requests to api.github.com and
// the Speakeasy caching proxy while counting the attempts. A successful
// install with zero blocked attempts proves the direct-URL fast path has no
// dependency on either.

// testContext mirrors what cmd/root.go always does at startup: stamp the
// running CLI version into the context. InstallVersion reads it for the
// version-equality short-circuit.
func testContext() context.Context {
	return events.SetSpeakeasyVersionInContext(context.Background(), "0.0.1")
}

func testArtifactArch() string {
	return runtime.GOOS + "_" + runtime.GOARCH
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// blockAPIHosts fails any request to the GitHub API or the Speakeasy caching
// proxy and returns a counter of blocked attempts.
func blockAPIHosts(t *testing.T) *atomic.Int64 {
	t.Helper()
	var attempts atomic.Int64
	original := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		host := r.URL.Hostname()
		if host == "api.github.com" || host == "cli-releases.speakeasy.com" {
			attempts.Add(1)
			return nil, fmt.Errorf("test harness: blocked request to %s (simulating rate limit)", host)
		}
		return original.RoundTrip(r)
	})
	t.Cleanup(func() { http.DefaultTransport = original })
	return &attempts
}

func TestManualInstallPinnedVersionWithAPIUnreachable(t *testing.T) {
	apiAttempts := blockAPIHosts(t)

	// A version old enough to fall outside the caching proxy's ~30-release
	// window, so neither the GitHub API nor the proxy could have served it.
	const pinnedVersion = "1.773.1"

	// Remove any cached install so the download path actually runs.
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	_ = os.RemoveAll(filepath.Join(home, ".speakeasy", pinnedVersion))

	dst, err := InstallVersion(testContext(), pinnedVersion, testArtifactArch(), 120)
	if err != nil {
		t.Fatalf("InstallVersion failed with API blocked — direct-URL fast path did not engage: %v", err)
	}

	if n := apiAttempts.Load(); n != 0 {
		t.Fatalf("expected zero GitHub API / caching proxy requests, but %d were attempted", n)
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("installed binary missing at %s: %v", dst, err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("installed binary at %s is not executable", dst)
	}

	out, err := exec.Command(dst, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("installed binary failed to run: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), pinnedVersion) {
		t.Fatalf("installed binary reports wrong version: %s", out)
	}
	t.Logf("installed %s with zero API requests and verified: %s", dst, strings.TrimSpace(string(out)))
}

func TestManualNonexistentVersionFallsBackAndErrors(t *testing.T) {
	apiAttempts := blockAPIHosts(t)

	_, err := InstallVersion(testContext(), "9.999.999", testArtifactArch(), 30)
	if err == nil {
		t.Fatal("expected error for nonexistent version")
	}
	if !strings.Contains(err.Error(), "failed to find release for version 9.999.999") {
		t.Fatalf("unexpected error shape: %v", err)
	}
	if apiAttempts.Load() == 0 {
		t.Fatal("expected the fallback chain to attempt the GitHub API after the direct download failed")
	}
	t.Logf("fallback chain engaged as expected (%d blocked API attempts): %v", apiAttempts.Load(), err)
}
