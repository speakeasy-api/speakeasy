package gram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/log"
)

// PackageInfo contains relevant fields from package.json
type PackageInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ReadPackageJSON reads and parses package.json from the given directory
func ReadPackageJSON(dir string) (*PackageInfo, error) {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read package.json: %w", err)
	}

	var pkg PackageInfo
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("failed to parse package.json: %w", err)
	}

	if pkg.Name == "" {
		return nil, fmt.Errorf("package.json missing 'name' field")
	}
	if pkg.Version == "" {
		return nil, fmt.Errorf("package.json missing 'version' field")
	}

	return &pkg, nil
}

// DeriveSlug extracts the package slug from a potentially scoped package name
// e.g., "@my-org/my-mcp-server" -> "my-mcp-server"
func DeriveSlug(packageName string) string {
	parts := strings.Split(packageName, "/")
	return parts[len(parts)-1]
}

func IsInstalled() bool {
	_, err := exec.LookPath("gram")
	return err == nil
}

func InstallCLI(ctx context.Context) error {
	l := log.From(ctx)
	l.Info("Installing Gram CLI...")

	cmd := exec.CommandContext(ctx, "bash", "-c", "curl -fsSL https://go.getgram.ai/cli.sh | bash")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	l.Info("Gram CLI installed successfully!")
	return Auth(ctx)
}

func CheckAuth(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "gram", "whoami")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("not authenticated with Gram: %w\n%s", err, string(output))
	}
	return nil
}

func Auth(ctx context.Context) error {
	l := log.From(ctx)
	l.Info("Opening browser for Gram authentication...")

	cmd := exec.CommandContext(ctx, "gram", "auth")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to authenticate with Gram: %w", err)
	}
	return nil
}

type PushResult struct {
	URL            string
	Version        string
	Slug           string
	IdempotencyKey string
	AlreadyExists  bool
}

func Push(ctx context.Context, dir, project string) (*PushResult, error) {
	l := log.From(ctx)

	// Read package.json for version and slug
	pkg, err := ReadPackageJSON(dir)
	if err != nil {
		return nil, err
	}

	slug := DeriveSlug(pkg.Name)
	idempotencyKey := fmt.Sprintf("%s@%s", slug, pkg.Version)

	l.Infof("Deploying %s@%s to Gram...", slug, pkg.Version)

	// Use gram push directly with idempotency key
	args := []string{"push", "--config", "gram.deploy.json", "--idempotency-key", idempotencyKey}
	if project != "" {
		args = append(args, "--project", project)
	}

	cmd := exec.CommandContext(ctx, "gram", args...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin

	var stdout, stderr bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("deployment failed: %w\n%s", err, stderr.String())
	}

	combinedOutput := stdout.String() + stderr.String()
	url := parseDeploymentURL(combinedOutput)

	// Check if this was an existing deployment (idempotency hit)
	// Gram returns successfully but doesn't create a new deployment
	alreadyExists := strings.Contains(combinedOutput, "already exists") ||
		strings.Contains(combinedOutput, "existing deployment")

	return &PushResult{
		URL:            url,
		Version:        pkg.Version,
		Slug:           slug,
		IdempotencyKey: idempotencyKey,
		AlreadyExists:  alreadyExists,
	}, nil
}

func parseDeploymentURL(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "https://") {
			start := strings.Index(line, "https://")
			if start >= 0 {
				url := line[start:]
				if idx := strings.IndexAny(url, " \t\n\r"); idx > 0 {
					url = url[:idx]
				}
				return url
			}
		}
	}
	return ""
}

func Build(ctx context.Context, dir string) error {
	l := log.From(ctx)
	l.Info("Building MCP server...")

	cmd := exec.CommandContext(ctx, "npm", "run", "build")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	l.Info("Building Gram deployment artifacts...")
	gramCmd := exec.CommandContext(ctx, "npm", "run", "gram:build")
	gramCmd.Dir = dir
	gramCmd.Stdout = os.Stdout
	gramCmd.Stderr = os.Stderr

	if err := gramCmd.Run(); err != nil {
		return fmt.Errorf("gram build failed: %w", err)
	}

	return nil
}
