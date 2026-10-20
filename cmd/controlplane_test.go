package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestControlPlaneCommandError(t *testing.T) {
	t.Parallel()

	newRoot := func(commands ...string) *cobra.Command {
		root := &cobra.Command{Use: "speakeasy", RunE: func(*cobra.Command, []string) error { return nil }}
		for _, name := range commands {
			root.AddCommand(&cobra.Command{Use: name, RunE: func(*cobra.Command, []string) error { return nil }})
		}
		return root
	}

	tests := []struct {
		name     string
		commands []string
		args     []string
		wantHint string
	}{
		{name: "push", commands: []string{"generate"}, args: []string{"push"}, wantHint: "push"},
		{name: "upload", commands: []string{"generate"}, args: []string{"upload"}, wantHint: "upload"},
		{name: "stage", commands: []string{"generate"}, args: []string{"stage"}, wantHint: "stage"},
		{name: "whoami", commands: []string{"generate"}, args: []string{"whoami"}, wantHint: "whoami"},
		{name: "functions with args and flags", commands: []string{"generate"}, args: []string{"functions", "build", "--help"}, wantHint: "functions"},
		{name: "redeploy", commands: []string{"generate"}, args: []string{"redeploy"}, wantHint: "redeploy"},
		{name: "install", commands: []string{"generate"}, args: []string{"install"}, wantHint: "install"},
		{name: "no args", commands: []string{"generate"}, args: nil},
		{name: "command of this CLI", commands: []string{"generate"}, args: []string{"generate", "sdk"}},
		{name: "unknown command", commands: []string{"generate"}, args: []string{"genrate"}},
		{name: "flag only", commands: []string{"generate"}, args: []string{"--version"}},
		{name: "this CLI has a command with the same name", commands: []string{"generate", "whoami"}, args: []string{"whoami"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := controlPlaneCommandError(newRoot(tt.commands...), tt.args)
			if tt.wantHint == "" {
				assert.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), `"`+tt.wantHint+`" is a command of the Speakeasy AI Control Plane CLI, not of this CLI.`)
		})
	}
}

func TestControlPlaneCommandErrorMessage(t *testing.T) {
	t.Parallel()

	root := &cobra.Command{Use: "speakeasy"}
	root.AddCommand(&cobra.Command{Use: "generate"})

	err := controlPlaneCommandError(root, []string{"functions"})
	require.EqualError(t, err, `"functions" is a command of the Speakeasy AI Control Plane CLI, not of this CLI. This speakeasy is the Speakeasy SDK generator. Both CLIs install a binary named speakeasy. To install the AI Control Plane CLI alongside it, see https://www.speakeasy.com/docs/ai-control-plane/reference/command-line`)
}
