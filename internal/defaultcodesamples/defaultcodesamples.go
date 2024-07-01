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
	tempDir := os.TempDir()
	tempFile := fmt.Sprintf("%s/defaultcodesamples.js", tempDir)
	err = os.WriteFile(tempFile, result, 0o644)
	if err != nil {
		return fmt.Errorf("failed to write default code samples file: %w", err)
	}

	cmd := exec.Command(
		nodeBinary,
		tempFile,
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
