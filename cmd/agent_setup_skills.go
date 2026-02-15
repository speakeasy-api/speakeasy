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

// agentConfig describes a coding agent and its skills directory.
type agentConfig struct {
	Name        string // display name
	Dir         string // skills directory relative to project root
	Preselected bool   // selected by default in the multi-select
}

// knownAgents lists all supported agents, matching the upstream skills registry.
// Agents marked Preselected are the most commonly used and selected by default.
// The .agents/skills directory is the canonical/universal location — always installed.
var knownAgents = []agentConfig{
	// Popular agents — preselected
	{Name: "Claude Code", Dir: ".claude/skills", Preselected: true},
	{Name: "Codex", Dir: ".agents/skills", Preselected: true},
	{Name: "Cursor", Dir: ".cursor/skills", Preselected: true},
	{Name: "Windsurf", Dir: ".codeium/windsurf/skills", Preselected: true},
	{Name: "Copilot", Dir: ".github/copilot/skills", Preselected: true},

	// Other agents (share .agents/skills universal dir)
	{Name: "Amp", Dir: ".agents/skills"},
	{Name: "Gemini CLI", Dir: ".agents/skills"},
	{Name: "Antigravity", Dir: ".agent/skills"},
	{Name: "Augment", Dir: ".augment/skills"},
	{Name: "Cline", Dir: ".cline/skills"},
	{Name: "CodeBuddy", Dir: ".codebuddy/skills"},
	{Name: "Command Code", Dir: ".commandcode/skills"},
	{Name: "Continue", Dir: ".continue/skills"},
	{Name: "Crush", Dir: ".crush/skills"},
	{Name: "Droid", Dir: ".factory/skills"},
	{Name: "Goose", Dir: ".goose/skills"},
	{Name: "iFlow", Dir: ".iflow/skills"},
	{Name: "Junie", Dir: ".junie/skills"},
	{Name: "Kilo", Dir: ".kilocode/skills"},
	{Name: "Kiro", Dir: ".kiro/skills"},
	{Name: "Kode", Dir: ".kode/skills"},
	{Name: "MCPJam", Dir: ".mcpjam/skills"},
	{Name: "Mux", Dir: ".mux/skills"},
	{Name: "Neovate", Dir: ".neovate/skills"},
	{Name: "OpenClaw", Dir: "skills"},
	{Name: "OpenHands", Dir: ".openhands/skills"},
	{Name: "Pi", Dir: ".pi/agent/skills"},
	{Name: "Pochi", Dir: ".pochi/skills"},
	{Name: "Qoder", Dir: ".qoder/skills"},
	{Name: "Qwen Code", Dir: ".qwen/skills"},
	{Name: "Roo", Dir: ".roo/skills"},
	{Name: "Trae", Dir: ".trae/skills"},
	{Name: "Vibe", Dir: ".vibe/skills"},
	{Name: "Zencoder", Dir: ".zencoder/skills"},
	{Name: "Adal", Dir: ".adal/skills"},
}

// defaultAgentDirs returns the directories for the preselected (default) agents.
func defaultAgentDirs() []string {
	seen := make(map[string]bool)
	var dirs []string
	// Always include the universal canonical dir
	dirs = append(dirs, ".agents/skills")
	seen[".agents/skills"] = true
	for _, a := range knownAgents {
		if a.Preselected && !seen[a.Dir] {
			dirs = append(dirs, a.Dir)
			seen[a.Dir] = true
		}
	}
	return dirs
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

// installSkillsNative writes skill files to the canonical location and creates
// symlinks in the specified agent directories.
func installSkillsNative(ctx context.Context, projectDir string, skillNames []string, agentDirs []string) error {
	canonicalDir := filepath.Join(projectDir, ".agents", "skills")

	for _, skillName := range skillNames {
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

		// Create symlinks from each selected agent's skills dir
		for _, agentDir := range agentDirs {
			if agentDir == ".agents/skills" {
				continue // canonical dir already has the file, no symlink needed
			}

			agentSkillsDir := filepath.Join(projectDir, agentDir)
			if err := os.MkdirAll(agentSkillsDir, 0o755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", agentSkillsDir, err)
			}

			linkPath := filepath.Join(agentSkillsDir, skillName)
			relTarget, err := filepath.Rel(agentSkillsDir, filepath.Join(canonicalDir, skillName))
			if err != nil {
				return fmt.Errorf("failed to compute relative path for symlink: %w", err)
			}

			_ = os.RemoveAll(linkPath)
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

		if err := installSkillsNative(ctx, projectDir, []string{"speakeasy-context"}, defaultAgentDirs()); err != nil {
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

	// --- Agent selection ---
	agentOptions := make([]huh.Option[string], 0, len(knownAgents))
	selectedAgentDirs := make([]string, 0)
	seen := make(map[string]bool)
	for _, a := range knownAgents {
		if seen[a.Dir] {
			continue // skip duplicate dirs (e.g. Amp/Codex/Gemini share .agents/skills)
		}
		seen[a.Dir] = true
		opt := huh.NewOption(a.Name, a.Dir)
		if a.Preselected {
			opt = opt.Selected(true)
			selectedAgentDirs = append(selectedAgentDirs, a.Dir)
		}
		agentOptions = append(agentOptions, opt)
	}

	agentForm := charm.NewForm(huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Which agents do you use?").
			Description("Skills will be installed for the selected agents.\n").
			Options(agentOptions...).
			Value(&selectedAgentDirs),
	)), charm.WithKey("x/space", "toggle"))

	if _, err := agentForm.ExecuteForm(); err != nil {
		return err
	}

	if len(selectedAgentDirs) == 0 {
		fmt.Println("No agents selected. Skipping skill installation.")
		return nil
	}

	// Always include canonical dir
	hasCanonical := false
	for _, d := range selectedAgentDirs {
		if d == ".agents/skills" {
			hasCanonical = true
			break
		}
	}
	if !hasCanonical {
		selectedAgentDirs = append([]string{".agents/skills"}, selectedAgentDirs...)
	}

	// --- Skill selection ---
	log.From(ctx).Infof("Fetching skill catalog from github.com/%s/%s...", skillsOwner, skillsRepo)
	skillNames, err := fetchSkillNames(ctx)
	if err != nil {
		return err
	}

	if len(skillNames) == 0 {
		log.From(ctx).Warnf("No skills found in the repository.")
		return nil
	}

	skillOptions := make([]huh.Option[string], 0, len(skillNames))
	selectedSkills := make([]string, 0)
	for _, name := range skillNames {
		opt := huh.NewOption(name, name)
		if name == "speakeasy-context" {
			opt = opt.Selected(true)
			selectedSkills = append(selectedSkills, name)
		}
		skillOptions = append(skillOptions, opt)
	}

	skillForm := charm.NewForm(huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Select skills to install").
			Description("These skills help AI coding assistants work with Speakeasy.\n").
			Options(skillOptions...).
			Value(&selectedSkills),
	)), charm.WithKey("x/space", "toggle"))

	if _, err := skillForm.ExecuteForm(); err != nil {
		return err
	}

	if len(selectedSkills) == 0 {
		fmt.Println("No skills selected. Skipping installation.")
		return nil
	}

	// --- Install ---
	if err := installSkillsNative(ctx, projectDir, selectedSkills, selectedAgentDirs); err != nil {
		return err
	}

	log.From(ctx).Successf("Installed %d skill(s) for %d agent(s): %s",
		len(selectedSkills), len(selectedAgentDirs), strings.Join(selectedSkills, ", "))
	return nil
}
