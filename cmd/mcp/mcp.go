package mcp

import (
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/utils"
)

const mcpLong = `# MCP
The ` + "`mcp`" + ` command provides utilities for managing MCP (Model Context Protocol) servers.
`

var MCPCmd = &model.CommandGroup{
	Usage:          "mcp",
	Short:          "Commands for MCP server management",
	Long:           utils.RenderMarkdown(mcpLong),
	InteractiveMsg: "What do you want to do?",
	Commands:       []model.Command{deployCmd},
	Hidden:         true,
}
