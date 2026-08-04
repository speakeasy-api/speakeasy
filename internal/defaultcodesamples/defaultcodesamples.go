package defaultcodesamples

import (
	"context"
	"embed"
	"fmt"
	"os"
	"os/exec"
)

type DefaultCodeSamplesFlags struct {
	SchemaPath string `json:"schema"`
	Language   string `json:"language"`
	Out        string `json:"out"`
}

//go:embed out/defaultcodesamples.js
var javascriptFile embed.FS

func DefaultCodeSamples(ctx context.Context, flags DefaultCodeSamplesFlags) error {
	// Ensure the user has node installed
	nodeBinary, err := findNodeBinary()
	if err != nil {
		return err
	}

	out, err := os.Create(flags.Out)
	if err != nil {
		return fmt.Errorf("failed to open output file: %w", err)
	}
	defer out.Close()

	// Copy the file to a temp location
	result, err := javascriptFile.ReadFile("out/defaultcodesamples.js")
	if err != nil {
		return fmt.Errorf("failed to read default code samples file: %w", err)
	}
	// Use a unique, owner-only temp file: a fixed world-readable path could be
	// pre-created or swapped by another local user before node executes it.
	tempFile, err := os.CreateTemp("", "speakeasy-defaultcodesamples-*.js")
	if err != nil {
		return fmt.Errorf("failed to create temp file for default code samples: %w", err)
	}
	defer os.Remove(tempFile.Name())
	if _, err := tempFile.Write(result); err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to write default code samples file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close default code samples file: %w", err)
	}

	cmd := exec.Command(
		nodeBinary,
		tempFile.Name(),
		"-s", flags.SchemaPath,
		"-l", flags.Language,
	)

	cmd.Stdout = out
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run command: %w", err)
	}

	return nil
}

func findNodeBinary() (string, error) {
	// Check if node is installed
	_, err := exec.Command("node", "--version").Output()
	if err == nil {
		return "node", nil
	}

	return "", fmt.Errorf("node is required to run this command. Please install node and try again")
}
