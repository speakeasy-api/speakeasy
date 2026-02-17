package git

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
)

// RunGitCommand executes a native git command in the given directory and returns its stdout.
// This is the shared helper for running native git commands that both the CLI and
// the GitHub Action can use. It captures stdout and stderr separately, returning
// stderr in the error message on failure.
func RunGitCommand(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run git %s: %w - %s", args[0], err, errb.String())
	}

	return outb.String(), nil
}

// RunGitCommandWithStdin executes a native git command with stdin input in the given directory.
func RunGitCommandWithStdin(dir string, stdin io.Reader, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdin = stdin
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run git %s: %w - %s", args[0], err, errb.String())
	}

	return outb.String(), nil
}

// RunGitCommandInRepo executes a native git command in the repository's root directory.
// Returns an error if the repository is not initialized.
func (r *Repository) RunGitCommandInRepo(args ...string) (string, error) {
	root := r.Root()
	if root == "" {
		return "", fmt.Errorf("repository root not found")
	}
	return RunGitCommand(root, args...)
}
