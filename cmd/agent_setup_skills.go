package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

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

var skillCatalog = []struct {
	ID          string
	Label       string
	Preselected bool
}{
	{"speakeasy-context", "Session bootstrap: always run agent context first", true},
	{"start-new-sdk-project", "Generate a new SDK from an OpenAPI spec", false},
	{"diagnose-generation-failure", "Diagnose SDK generation errors", false},
	{"configure-sdk-options", "Configure gen.yaml for SDK targets", false},
	{"writing-openapi-specs", "Author or improve OpenAPI specs", false},
	{"manage-openapi-overlays", "Create and apply OpenAPI overlays", false},
	{"improve-sdk-naming", "AI-powered SDK naming suggestions", false},
	{"generate-terraform-provider", "Generate Terraform provider", false},
	{"extract-openapi-from-code", "Extract OpenAPI spec from code", false},
	{"customize-sdk-hooks", "Add custom SDK lifecycle hooks", false},
	{"setup-sdk-testing", "Set up SDK contract/integration tests", false},
	{"generate-mcp-server", "Generate an MCP server", false},
	{"customize-sdk-runtime", "Configure retries, timeouts, pagination", false},
	{"orchestrate-multi-repo-sdks", "Multi-repo SDK management", false},
	{"orchestrate-multi-target-sdks", "Multi-language SDK generation", false},
}

func checkNpxAvailable() error {
	_, err := exec.LookPath("npx")
	if err != nil {
		return fmt.Errorf("npx is required to install agent skills but was not found.\n\n" +
			"Install Node.js from: https://nodejs.org/\n\n" +
			"Then re-run: speakeasy agent setup-skills")
	}
	return nil
}

func installSkills(ctx context.Context, skills []string) error {
	if err := checkNpxAvailable(); err != nil {
		return err
	}

	args := []string{"--yes", "skills", "add"}
	for _, s := range skills {
		args = append(args, fmt.Sprintf("speakeasy-api/skills/%s", s))
	}

	cmd := exec.CommandContext(ctx, "npx", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func runSetupSkills(ctx context.Context, flags AgentSetupSkillsFlags) error {
	if flags.Auto {
		defaultSkills := []string{}
		for _, s := range skillCatalog {
			if s.Preselected {
				defaultSkills = append(defaultSkills, s.ID)
			}
		}
		if err := installSkills(ctx, defaultSkills); err != nil {
			return err
		}
		log.From(ctx).Successf("Installed default agent skills: %s", strings.Join(defaultSkills, ", "))
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

	options := make([]huh.Option[string], 0, len(skillCatalog))
	selectedSkills := make([]string, 0)
	for _, s := range skillCatalog {
		opt := huh.NewOption(fmt.Sprintf("%s — %s", s.ID, s.Label), s.ID)
		if s.Preselected {
			opt = opt.Selected(true)
			selectedSkills = append(selectedSkills, s.ID)
		}
		options = append(options, opt)
	}

	form := charm.NewForm(huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Select Speakeasy agent skills to install").
			Description("These skills help AI coding assistants work with Speakeasy.\n").
			Options(options...).
			Value(&selectedSkills),
	)), charm.WithKey("x/space", "toggle"))

	if _, err := form.ExecuteForm(); err != nil {
		return err
	}

	if len(selectedSkills) == 0 {
		fmt.Println("Skipping skill installation.")
		return nil
	}

	if err := installSkills(ctx, selectedSkills); err != nil {
		return err
	}

	log.From(ctx).Successf("Installed %d agent skill(s): %s", len(selectedSkills), strings.Join(selectedSkills, ", "))
	return nil
}
