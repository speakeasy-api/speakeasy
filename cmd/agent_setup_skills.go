package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/speakeasy-api/huh"
	"github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
)

type AgentSetupSkillsFlags struct {
	Auto bool `json:"auto"`
}

var agentSetupSkillsCmd = &model.ExecutableCommand[AgentSetupSkillsFlags]{
	Usage:          "setup-skills",
	Short:          "Install Speakeasy agent skills for AI coding assistants",
	Long:           "Install companion skills for AI coding assistants (Claude Code, Cursor, etc.) from the Speakeasy skills package.",
	Run:            runSetupSkills,
	RunInteractive: runSetupSkillsInteractive,
	Flags: []flag.Flag{
		flag.BooleanFlag{
			Name:        "auto",
			Description: "Install only default skills (speakeasy-context) without prompting",
		},
	},
}

const skillsRepo = "speakeasy-api/skills"

func checkNpxAvailable() error {
	_, err := exec.LookPath("npx")
	if err != nil {
		return fmt.Errorf("npx is required to install agent skills but was not found.\n\n" +
			"Install Node.js from: https://nodejs.org/\n\n" +
			"Then re-run: speakeasy agent setup-skills")
	}
	return nil
}

func runNpxSkills(ctx context.Context, args ...string) error {
	fullArgs := append([]string{"--yes", "skills"}, args...)

	log.From(ctx).Infof("Running: npx --yes skills %s", joinArgs(args))

	cmd := exec.CommandContext(ctx, "npx", fullArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

func joinArgs(args []string) string {
	result := ""
	for i, a := range args {
		if i > 0 {
			result += " "
		}
		result += a
	}
	return result
}

func runSetupSkills(ctx context.Context, flags AgentSetupSkillsFlags) error {
	if flags.Auto {
		if err := checkNpxAvailable(); err != nil {
			return err
		}

		fmt.Println("This will use npx (Node.js) to install agent skills from github.com/" + skillsRepo)
		fmt.Println()

		if err := runNpxSkills(ctx, "add", skillsRepo, "--skill", "speakeasy-context", "--agent", "*", "-y"); err != nil {
			return err
		}
		log.From(ctx).Successf("Installed default agent skill: speakeasy-context")
		return nil
	}

	fmt.Println("Run in interactive mode or use --auto to install default skills:")
	fmt.Println("  speakeasy agent setup-skills        (interactive)")
	fmt.Println("  speakeasy agent setup-skills --auto  (default skills only)")
	return nil
}

func runSetupSkillsInteractive(ctx context.Context, flags AgentSetupSkillsFlags) error {
	if flags.Auto {
		return runSetupSkills(ctx, flags)
	}

	if err := checkNpxAvailable(); err != nil {
		log.From(ctx).Warnf("%s", err.Error())
		return nil
	}

	fmt.Println("Speakeasy offers companion skills for AI coding assistants (Claude Code, Cursor, etc.).")
	fmt.Println("This will use npx (Node.js) to install skills from github.com/" + skillsRepo)
	fmt.Println()

	proceed := true
	form := charm.NewForm(huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title("Would you like to install agent skills?").
			Value(&proceed),
	)))

	if _, err := form.ExecuteForm(); err != nil {
		return err
	}

	if !proceed {
		fmt.Println("Skipping skill installation.")
		return nil
	}

	// Delegate skill selection to the skills CLI — it fetches the catalog
	// directly from the repo so there's no duplicate list to maintain.
	return runNpxSkills(ctx, "add", skillsRepo)
}
