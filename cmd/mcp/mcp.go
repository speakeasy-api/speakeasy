package mcp

import (
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/utils"
)

const mcpLong = `# MCP
The ` + "`mcp`" + ` command provides utilities for managing MCP (Model Context Protocol) servers.

Commands:
- ` + "`deploy`" + ` - Deploy an MCP server to Gram

Use these commands to deploy and manage your generated MCP servers.
`

var MCPCmd = &model.CommandGroup{
	Usage:          "mcp",
	Short:          "Commands for MCP server management",
	Long:           utils.RenderMarkdown(mcpLong),
	InteractiveMsg: "What do you want to do?",
	Commands:       []model.Command{deployCmd},
}
