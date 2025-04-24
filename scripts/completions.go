package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	// Remove existing completions directory
	os.RemoveAll("completions")

	// Create new completions directory
	err := os.Mkdir("completions", 0755)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating completions directory: %v\n", err)
		os.Exit(1)
	}

	// Generate completions for different shells
	shells := []string{"bash", "zsh", "fish"}
	for _, shell := range shells {
		outputFile := filepath.Join("completions", "speakeasy."+shell)
		cmd := exec.Command("go", "run", "main.go", "completion", shell)
		output, err := cmd.Output()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating %s completion: %v\n", shell, err)
			os.Exit(1)
		}

		err = os.WriteFile(outputFile, output, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to %s: %v\n", outputFile, err)
			os.Exit(1)
		}
	}
}
