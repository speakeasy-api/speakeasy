package gram

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/log"
)

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
	URL string
}

func Push(ctx context.Context, dir, project string) (*PushResult, error) {
	l := log.From(ctx)

	args := []string{"gf", "push"}
	if project != "" {
		args = append(args, "--project", project)
		l.Infof("Deploying to Gram project: %s", project)
	} else {
		l.Info("Deploying to Gram (using default project from Gram auth)")
	}

	cmd := exec.CommandContext(ctx, "npx", args...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin

	var stdout, stderr bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("deployment failed: %w\n%s", err, stderr.String())
	}

	url := parseDeploymentURL(stdout.String())
	if url == "" {
		url = parseDeploymentURL(stderr.String())
	}

	return &PushResult{URL: url}, nil
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
