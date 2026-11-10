package cmd

import (
	"fmt"
	"slices"

	"github.com/spf13/cobra"
)

const controlPlaneCLIDocsURL = "https://www.speakeasy.com/docs/ai-control-plane/reference/command-line"

// controlPlaneOnlyCommands are top-level commands of the Speakeasy AI Control
// Plane CLI that this CLI does not have. Both CLIs install a binary named
// speakeasy, so people can run this one while they mean the other one.
var controlPlaneOnlyCommands = []string{"push", "upload", "stage", "whoami", "functions", "redeploy", "install"}

// controlPlaneCommandError returns an error that points to the Speakeasy AI
// Control Plane CLI when args start with one of its commands and root has no
// command with that name. It returns nil for all other args.
func controlPlaneCommandError(root *cobra.Command, args []string) error {
	if len(args) == 0 || !slices.Contains(controlPlaneOnlyCommands, args[0]) {
		return nil
	}

	if found, _, _ := root.Find(args); found != root {
		return nil
	}

	return fmt.Errorf("%q is a command of the Speakeasy AI Control Plane CLI, not of this CLI. This speakeasy is the Speakeasy SDK generator. Both CLIs install a binary named speakeasy. To install the AI Control Plane CLI alongside it, see %s", args[0], controlPlaneCLIDocsURL)
}
