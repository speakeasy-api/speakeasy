package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBasicAuth_WithToken(t *testing.T) {
	t.Parallel()

	auth := BasicAuth("ghp_test123")
	if auth == nil {
		t.Fatal("expected non-nil auth")
	}
	if auth.Username != "gen" {
		t.Errorf("expected username 'gen', got %q", auth.Username)
	}
	if auth.Password != "ghp_test123" {
		t.Errorf("expected password 'ghp_test123', got %q", auth.Password)
	}
}

func TestBasicAuth_EmptyToken(t *testing.T) {
	t.Parallel()

	auth := BasicAuth("")
	if auth != nil {
		t.Errorf("expected nil auth for empty token, got %+v", auth)
	}
}

func TestConfigureURLRewrite_EmptyToken(t *testing.T) {
	t.Parallel()

	err := ConfigureURLRewrite("/tmp", "github.com", "")
	if err != nil {
		t.Errorf("expected no error for empty token, got %v", err)
	}
}

func TestConfigureURLRewrite_SetsConfig(t *testing.T) {
	t.Parallel()

	// Create a temporary git repo for the test
	dir := t.TempDir()
	_, err := RunGitCommand(dir, "init")
	if err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	err = ConfigureURLRewrite(dir, "github.com", "test-token")
	if err != nil {
		t.Fatalf("ConfigureURLRewrite failed: %v", err)
	}

	// Verify the config was set
	output, err := RunGitCommand(dir, "config", "--local", "--get", "url.https://gen:test-token@github.com/.insteadOf")
	if err != nil {
		t.Fatalf("failed to read git config: %v", err)
	}
	expected := "https://github.com/\n"
	if output != expected {
		t.Errorf("expected config value %q, got %q", expected, output)
	}

	// Verify the token doesn't leak into .git/config in an unexpected way
	configPath := filepath.Join(dir, ".git", "config")
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read .git/config: %v", err)
	}
	config := string(configBytes)
	if !strings.Contains(config, "insteadOf") {
		t.Error("expected .git/config to contain insteadOf rule")
	}
}
