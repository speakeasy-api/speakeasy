package ci

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
)

type setupEnvFlags struct {
	Target        string `json:"target"`
	PnpmVersion   string `json:"pnpm-version"`
	PoetryVersion string `json:"poetry-version"`
	UvVersion     string `json:"uv-version"`
}

var setupEnvCmd = &model.ExecutableCommand[setupEnvFlags]{
	Usage: "setup-env",
	Short: "Install language-specific build dependencies (used by CI/CD)",
	Long: `Installs build dependencies needed for SDK compilation and testing.

When --target is specified, only installs dependencies for that language:
  - python/pythonv2: installs poetry and uv via pipx
  - typescript/typescriptv2: installs pnpm via npm (if --pnpm-version set)
  - Other targets: no additional dependencies needed

When no --target is specified, installs all dependencies (backward compatible).

Version flags allow pinning dependency versions:
  --pnpm-version 8.15.0
  --poetry-version 1.7.1
  --uv-version 0.1.0`,
	Run: runSetupEnv,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:         "target",
			Description:  "SDK target language (python, typescript, etc.)",
			DefaultValue: os.Getenv("INPUT_TARGET"),
		},
		flag.StringFlag{
			Name:         "pnpm-version",
			Description:  "Version of pnpm to install (TypeScript targets)",
			DefaultValue: os.Getenv("INPUT_PNPM_VERSION"),
		},
		flag.StringFlag{
			Name:         "poetry-version",
			Description:  "Version of poetry to install (Python targets)",
			DefaultValue: os.Getenv("INPUT_POETRY_VERSION"),
		},
		flag.StringFlag{
			Name:         "uv-version",
			Description:  "Version of uv to install (Python targets)",
			DefaultValue: os.Getenv("INPUT_UV_VERSION"),
		},
	},
}

func runSetupEnv(ctx context.Context, flags setupEnvFlags) error {
	logger := log.From(ctx)

	target := strings.ToLower(flags.Target)

	installPython := target == "" || strings.Contains(target, "python")
	installTS := target == "" || strings.Contains(target, "typescript")

	if installPython {
		if err := installPoetry(logger, flags.PoetryVersion); err != nil {
			return fmt.Errorf("failed to install poetry: %w", err)
		}
		if err := installUv(logger, flags.UvVersion); err != nil {
			return fmt.Errorf("failed to install uv: %w", err)
		}
	}

	if installTS && flags.PnpmVersion != "" {
		if err := installPnpm(logger, flags.PnpmVersion); err != nil {
			return fmt.Errorf("failed to install pnpm: %w", err)
		}
	}

	if !installPython && !installTS {
		logger.Infof("No additional dependencies needed for target: %s", target)
	}

	return nil
}

func installPoetry(logger log.Logger, version string) error {
	poetrySpec := "poetry"
	if version != "" {
		poetrySpec = "poetry==" + version
	}
	logger.Infof("Installing %s...", poetrySpec)
	cmd := exec.Command("pipx", "install", "--global", poetrySpec)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func installUv(logger log.Logger, version string) error {
	uvSpec := "uv"
	if version != "" {
		uvSpec = "uv==" + version
	}
	logger.Infof("Installing %s...", uvSpec)
	cmd := exec.Command("pipx", "install", "--global", uvSpec)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func installPnpm(logger log.Logger, version string) error {
	pnpmSpec := "pnpm@" + version
	logger.Infof("Installing %s...", pnpmSpec)
	cmd := exec.Command("npm", "install", "-g", pnpmSpec)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
