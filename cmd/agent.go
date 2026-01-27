package cmd

import (
	"context"

	agentcontent "github.com/speakeasy-api/speakeasy-agent-mode-content"
	"github.com/speakeasy-api/speakeasy/internal/agent"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/spf13/cobra"
)

var agentCmd = &model.CommandGroup{
	Usage:          "agent",
	Short:          "Agent-related tools and documentation",
	Long:           "Commands for AI agents interacting with Speakeasy.",
	InteractiveMsg: "What would you like to do?",
	Commands:       []model.Command{agentContextCmd, agentFeedbackCmd},
}

type AgentContextFlags struct {
	Path    string `json:"path"`
	Grep    string `json:"grep"`
	Regex   bool   `json:"regex"`
	Context int    `json:"context"`
	After   int    `json:"after"`
	Before  int    `json:"before"`
	List    bool   `json:"list"`
	JSON    bool   `json:"json"`
}

var agentContextCmd = &model.ExecutableCommand[AgentContextFlags]{
	Usage: "context [path]",
	Short: "Browse agent context documentation",
	Long:  "Browse the embedded agent context filesystem. Provides AI agents with structured access to Speakeasy documentation.",
	Run:   runAgentContext,
	PreRun: func(cmd *cobra.Command, flags *AgentContextFlags) error {
		args := cmd.Flags().Args()
		if len(args) > 0 && flags.Path == "" {
			flags.Path = args[0]
		}
		return nil
	},
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "grep",
			Shorthand:   "g",
			Description: "Search content for a literal string",
		},
		flag.BooleanFlag{
			Name:        "regex",
			Shorthand:   "e",
			Description: "Treat --grep pattern as a regular expression",
		},
		flag.IntFlag{
			Name:         "context",
			Shorthand:    "C",
			Description:  "Lines of context around grep matches",
			DefaultValue: 2,
		},
		flag.IntFlag{
			Name:        "after",
			Shorthand:   "A",
			Description: "Lines of context after grep matches (overrides -C for after)",
		},
		flag.IntFlag{
			Name:        "before",
			Shorthand:   "B",
			Description: "Lines of context before grep matches (overrides -C for before)",
		},
		flag.BooleanFlag{
			Name:        "list",
			Shorthand:   "l",
			Description: "List matching file paths only (with --grep), or list all doc paths (without --grep)",
		},
		flag.BooleanFlag{
			Name:        "json",
			Description: "Output in JSON format",
		},
	},
}

func runAgentContext(ctx context.Context, flags AgentContextFlags) error {
	contentFS := agentcontent.FS()

	p, err := agent.NormalizePath(flags.Path)
	if err != nil {
		return err
	}

	if flags.List && flags.Grep == "" {
		return agent.ListAll(contentFS, flags.JSON)
	}

	if flags.Grep != "" {
		beforeCount := flags.Context
		afterCount := flags.Context
		if flags.Before > 0 {
			beforeCount = flags.Before
		}
		if flags.After > 0 {
			afterCount = flags.After
		}
		return agent.Grep(contentFS, agent.GrepOptions{
			ScopePath:  p,
			Pattern:    flags.Grep,
			IsRegex:    flags.Regex,
			Before:     beforeCount,
			After:      afterCount,
			ListOnly:   flags.List,
			JSONOutput: flags.JSON,
		})
	}

	result, err := agent.ResolvePath(contentFS, p)
	if err != nil {
		return err
	}

	if result.IsDir {
		return agent.ListDir(contentFS, result.ResolvedPath, flags.JSON)
	}

	return agent.ReadFile(contentFS, result.ResolvedPath, p, flags.JSON)
}

type AgentFeedbackFlags struct {
	Message     string `json:"message"`
	Type        string `json:"type"`
	ContextPath string `json:"context-path"`
}

var agentFeedbackCmd = &model.ExecutableCommand[AgentFeedbackFlags]{
	Usage: "feedback",
	Short: "Submit feedback on agent context content",
	Long:  "Submit feedback on Speakeasy agent context documentation. Feedback is sent anonymously unless you are authenticated.",
	Run:   runAgentFeedback,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "message",
			Shorthand:   "m",
			Description: "The feedback message",
			Required:    true,
		},
		flag.StringFlag{
			Name:         "type",
			Shorthand:    "t",
			Description:  "Feedback type: agent_context or general",
			DefaultValue: "agent_context",
		},
		flag.StringFlag{
			Name:        "context-path",
			Description: "The agent-context path this feedback relates to",
		},
	},
}

func runAgentFeedback(ctx context.Context, flags AgentFeedbackFlags) error {
	return agent.SubmitFeedback(ctx, flags.Type, flags.Message, flags.ContextPath)
}
