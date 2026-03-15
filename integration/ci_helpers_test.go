package integration_tests

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func executeWithEnv(t *testing.T, wd string, envOverrides map[string]string, args ...string) Runnable {
	t.Helper()

	binaryPath, err := ensureBinary()
	require.NoError(t, err, "failed to build speakeasy binary")

	execCmd := exec.Command(binaryPath, args...)
	execCmd.Env = mergeEnv(os.Environ(), envOverrides)
	execCmd.Dir = wd

	out := bytes.Buffer{}
	execCmd.Stdout = &out
	execCmd.Stderr = &out

	return &subprocessRunner{
		cmd: execCmd,
		out: &out,
	}
}

func executeCIWithEnv(t *testing.T, wd string, envOverrides map[string]string, args ...string) Runnable {
	t.Helper()

	baseEnv := map[string]string{
		"GITHUB_WORKSPACE":  wd,
		"GITHUB_OUTPUT":     filepath.Join(wd, "github-output.txt"),
		"GITHUB_SERVER_URL": defaultString(os.Getenv("GITHUB_SERVER_URL"), "https://github.com"),
		"GITHUB_REPOSITORY": defaultString(os.Getenv("GITHUB_REPOSITORY"), "test-org/test-repo"),
		"GITHUB_REPOSITORY_OWNER": defaultString(
			os.Getenv("GITHUB_REPOSITORY_OWNER"),
			"test-org",
		),
	}

	for key, value := range envOverrides {
		baseEnv[key] = value
	}

	return executeWithEnv(t, wd, baseEnv, append([]string{"ci"}, args...)...)
}

func mergeEnv(base []string, overrides map[string]string) []string {
	envMap := map[string]string{}
	for _, entry := range base {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	for key, value := range overrides {
		envMap[key] = value
	}

	merged := make([]string, 0, len(envMap))
	for key, value := range envMap {
		merged = append(merged, key+"="+value)
	}

	return merged
}

func defaultString(value, fallback string) string {
	if value != "" {
		return value
	}

	return fallback
}
