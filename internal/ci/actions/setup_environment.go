package actions

import (
	"fmt"
	"os/exec"

	"github.com/speakeasy-api/speakeasy/internal/ci/environment"
)

// SetupEnvironment will install runtime environment dependencies.
//
// For example if pnpm is desired instead of npm for target compilation and
// publishing, then an input (pnpm_version in this case) should be set to a
// non-empty value and this logic will install the dependency.
func SetupEnvironment() error {
	if err := installPoetry(); err != nil {
		return err
	}

	if err := installUv(); err != nil {
		return err
	}

	if err := installBundler(); err != nil {
		return err
	}

	if pnpmVersion := environment.GetPnpmVersion(); pnpmVersion != "" {
		pnpmPackageSpec := "pnpm@" + pnpmVersion
		cmd := exec.Command("npm", "install", "-g", pnpmPackageSpec)

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("error installing %s: %w", pnpmPackageSpec, err)
		}
	}

	return nil
}

// Installs poetry using pipx. If the INPUT_POETRY_VERSION environment variable
// is set, it will install that version.
func installPoetry() error {
	poetrySpec := "poetry"

	if poetryVersion := environment.GetPoetryVersion(); poetryVersion != "" {
		poetrySpec = "poetry==" + poetryVersion
	}

	cmd := exec.Command("pipx", "install", "--global", poetrySpec)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error installing poetry: %w", err)
	}

	return nil
}

// installBundler ensures the Ruby bundler gem is available.
// ubuntu-24.04 runners have Ruby pre-installed but bundler is not on PATH.
func installBundler() error {
	if _, err := exec.LookPath("bundle"); err == nil {
		return nil // already available
	}

	cmd := exec.Command("gem", "install", "bundler")
	if err := cmd.Run(); err != nil {
		// Non-fatal: only Ruby targets need bundler, and gem may not be installed.
		return nil
	}

	return nil
}

// Installs uv using pipx. If the INPUT_UV_VERSION environment variable
// is set, it will install that version.
func installUv() error {
	uvSpec := "uv"

	if uvVersion := environment.GetUvVersion(); uvVersion != "" {
		uvSpec = "uv==" + uvVersion
	}

	cmd := exec.Command("pipx", "install", "--global", uvSpec)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error installing uv: %w", err)
	}

	return nil
}
