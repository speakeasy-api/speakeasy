package gram

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	gramcmd "github.com/speakeasy-api/gram/cli/pkg/cmd"
	"github.com/speakeasy-api/speakeasy/internal/log"
)

func withGramLogger(ctx context.Context) context.Context {
	return gramcmd.PushLogger(ctx, slog.Default())
}

func loadProfile() (*gramcmd.Profile, error) {
	profilePath, err := gramcmd.DefaultProfilePath()
	if err != nil {
		return nil, err
	}
	return gramcmd.LoadProfile(profilePath)
}

type PackageInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

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

func DeriveSlug(packageName string) string {
	parts := strings.Split(packageName, "/")
	return parts[len(parts)-1]
}

func IsInstalled() bool {
	return true
}

func InstallCLI(ctx context.Context) error {
	return Auth(ctx)
}

func CheckAuth(ctx context.Context) error {
	prof, err := loadProfile()
	if err != nil {
		return fmt.Errorf("not authenticated with Gram: %w", err)
	}
	_, err = gramcmd.Whoami(withGramLogger(ctx), gramcmd.WhoamiOptions{
		Profile: prof,
	})
	if err != nil {
		return fmt.Errorf("not authenticated with Gram: %w", err)
	}
	return nil
}

func Auth(ctx context.Context) error {
	l := log.From(ctx)
	l.Info("Opening browser for Gram authentication...")

	prof, _ := loadProfile()
	_, err := gramcmd.Auth(withGramLogger(ctx), gramcmd.AuthOptions{
		Profile: prof,
	})
	if err != nil {
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

	pkg, err := ReadPackageJSON(dir)
	if err != nil {
		return nil, err
	}

	slug := DeriveSlug(pkg.Name)
	idempotencyKey := fmt.Sprintf("%s@%s", slug, pkg.Version)

	l.Infof("Deploying %s@%s to Gram...", slug, pkg.Version)

	configPath := filepath.Join(dir, "gram.deploy.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("gram.deploy.json not found in %s", dir)
	}

	prof, err := loadProfile()
	if err != nil {
		return nil, fmt.Errorf("not authenticated with Gram: %w", err)
	}

	result, err := gramcmd.Push(withGramLogger(ctx), gramcmd.PushOptions{
		Profile:        prof,
		ConfigFile:     configPath,
		ProjectSlug:    project,
		IdempotencyKey: idempotencyKey,
		Method:         "merge",
	})
	if err != nil {
		return nil, fmt.Errorf("deployment failed: %w", err)
	}

	return &PushResult{
		URL:            result.LogsURL,
		Version:        pkg.Version,
		Slug:           slug,
		IdempotencyKey: idempotencyKey,
	}, nil
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

	// Stage the function to create gram.deploy.json
	zipPath := filepath.Join(dir, "dist", "gram.zip")
	if err := StageFunction(ctx, dir, zipPath); err != nil {
		return fmt.Errorf("failed to stage function: %w", err)
	}

	return nil
}

// StageFunction stages a Gram Functions zip file for deployment.
// It creates or updates the gram.deploy.json config file.
func StageFunction(ctx context.Context, dir string, zipLocation string) error {
	l := log.From(ctx)

	pkg, err := ReadPackageJSON(dir)
	if err != nil {
		return err
	}

	slug := DeriveSlug(pkg.Name)
	configPath := filepath.Join(dir, "gram.deploy.json")

	l.Infof("Staging function %s from %s...", slug, zipLocation)

	if err := gramcmd.StageFunction(gramcmd.StageFunctionOptions{
		ConfigFile: configPath,
		Slug:       slug,
		Name:       pkg.Name,
		Location:   zipLocation,
		Runtime:    "nodejs:22",
	}); err != nil {
		return fmt.Errorf("failed to stage function: %w", err)
	}

	return nil
}

// StageOpenAPI stages an OpenAPI document for deployment.
// It creates or updates the gram.deploy.json config file.
func StageOpenAPI(ctx context.Context, dir string, specLocation string, slug string, name string) error {
	l := log.From(ctx)

	configPath := filepath.Join(dir, "gram.deploy.json")

	l.Infof("Staging OpenAPI spec %s from %s...", slug, specLocation)

	if err := gramcmd.StageOpenAPI(gramcmd.StageOpenAPIOptions{
		ConfigFile: configPath,
		Slug:       slug,
		Name:       name,
		Location:   specLocation,
	}); err != nil {
		return fmt.Errorf("failed to stage OpenAPI spec: %w", err)
	}

	return nil
}
