package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

const (
	skillsOwner = "speakeasy-api"
	skillsRepo  = "skills"
	skillsPath  = "skills" // directory in the repo containing skills
)

// agentDirs lists well-known agent skill directory paths (relative to project root).
// .agents/skills is the canonical location; the rest are symlinked to it.
var agentDirs = []string{
	".adal/skills",
	".agent/skills",
	".augment/skills",
	".claude/skills",
	".cline/skills",
	".codebuddy/skills",
	".commandcode/skills",
	".continue/skills",
	".crush/skills",
	".cursor/skills",
	".factory/skills",
	".goose/skills",
	".iflow/skills",
	".junie/skills",
	".kilocode/skills",
	".kiro/skills",
	".kode/skills",
	".mcpjam/skills",
	".mux/skills",
	".neovate/skills",
	".openhands/skills",
	".pi/skills",
	".pochi/skills",
	".qoder/skills",
	".qwen/skills",
	".roo/skills",
	".trae/skills",
	".vibe/skills",
	".windsurf/skills",
	".zencoder/skills",
	"skills",
}

// githubEntry represents a file/dir entry from the GitHub Contents API.
type githubEntry struct {
	Name string `json:"name"`
	Type string `json:"type"` // "file" or "dir"
}

// fetchSkillNames returns the list of skill directory names from the GitHub repo.
func fetchSkillNames(ctx context.Context) ([]string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", skillsOwner, skillsRepo, skillsPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch skill catalog from GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d when fetching skill catalog", resp.StatusCode)
	}

	var entries []githubEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("failed to parse skill catalog response: %w", err)
	}

	var names []string
	for _, e := range entries {
		if e.Type == "dir" {
			names = append(names, e.Name)
		}
	}
	return names, nil
}

// fetchSkillContent downloads the raw SKILL.md for a given skill name.
func fetchSkillContent(ctx context.Context, skillName string) ([]byte, error) {
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/master/%s/%s/SKILL.md", skillsOwner, skillsRepo, skillsPath, skillName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download %s/SKILL.md: %w", skillName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub returned status %d for %s/SKILL.md", resp.StatusCode, skillName)
	}

	return io.ReadAll(resp.Body)
}

// installSkillsNative writes skill files and creates symlinks for all known agents.
func installSkillsNative(ctx context.Context, projectDir string, skills []string) error {
	canonicalDir := filepath.Join(projectDir, ".agents", "skills")

	for _, skillName := range skills {
		log.From(ctx).Infof("Fetching skill: %s", skillName)

		content, err := fetchSkillContent(ctx, skillName)
		if err != nil {
			return err
		}

		// Write to canonical location: .agents/skills/<name>/SKILL.md
		skillDir := filepath.Join(canonicalDir, skillName)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", skillDir, err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), content, 0o644); err != nil {
			return fmt.Errorf("failed to write %s/SKILL.md: %w", skillName, err)
		}

		// Create symlinks from each agent's skills dir
		for _, agentDir := range agentDirs {
			agentSkillsDir := filepath.Join(projectDir, agentDir)
			if err := os.MkdirAll(agentSkillsDir, 0o755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", agentSkillsDir, err)
			}

			linkPath := filepath.Join(agentSkillsDir, skillName)
			// Compute relative path from agent skills dir to canonical location
			relTarget, err := filepath.Rel(agentSkillsDir, filepath.Join(canonicalDir, skillName))
			if err != nil {
				return fmt.Errorf("failed to compute relative path for symlink: %w", err)
			}

			// Remove existing symlink/dir if present, then create
			os.RemoveAll(linkPath)
			if err := os.Symlink(relTarget, linkPath); err != nil {
				return fmt.Errorf("failed to create symlink %s: %w", linkPath, err)
			}
		}
	}

	return nil
}

func runSetupSkills(ctx context.Context, flags AgentSetupSkillsFlags) error {
	if flags.Auto {
		projectDir, err := os.Getwd()
		if err != nil {
			return err
		}

		if err := installSkillsNative(ctx, projectDir, []string{"speakeasy-context"}); err != nil {
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

	projectDir, err := os.Getwd()
	if err != nil {
		return err
	}

	fmt.Println("Speakeasy offers companion skills for AI coding assistants (Claude Code, Cursor, etc.).")
	fmt.Println("Skills will be fetched from github.com/" + skillsOwner + "/" + skillsRepo + " and installed locally.")
	fmt.Println()

	proceed := true
	confirmForm := charm.NewForm(huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title("Would you like to install agent skills?").
			Value(&proceed),
	)))

	if _, err := confirmForm.ExecuteForm(); err != nil {
		return err
	}

	if !proceed {
		fmt.Println("Skipping skill installation.")
		return nil
	}

	// Fetch available skills from the repo
	log.From(ctx).Infof("Fetching skill catalog from github.com/%s/%s...", skillsOwner, skillsRepo)
	skillNames, err := fetchSkillNames(ctx)
	if err != nil {
		return err
	}

	if len(skillNames) == 0 {
		log.From(ctx).Warnf("No skills found in the repository.")
		return nil
	}

	// Build options with speakeasy-context preselected
	options := make([]huh.Option[string], 0, len(skillNames))
	selectedSkills := make([]string, 0)
	for _, name := range skillNames {
		opt := huh.NewOption(name, name)
		if name == "speakeasy-context" {
			opt = opt.Selected(true)
			selectedSkills = append(selectedSkills, name)
		}
		options = append(options, opt)
	}

	selectForm := charm.NewForm(huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Select skills to install").
			Description("These skills help AI coding assistants work with Speakeasy.\n").
			Options(options...).
			Value(&selectedSkills),
	)), charm.WithKey("x/space", "toggle"))

	if _, err := selectForm.ExecuteForm(); err != nil {
		return err
	}

	if len(selectedSkills) == 0 {
		fmt.Println("Skipping skill installation.")
		return nil
	}

	if err := installSkillsNative(ctx, projectDir, selectedSkills); err != nil {
		return err
	}

	log.From(ctx).Successf("Installed %d skill(s): %s", len(selectedSkills), strings.Join(selectedSkills, ", "))
	return nil
}
