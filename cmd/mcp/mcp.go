package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/utils"
)

const mcpLong = `# MCP Server

The ` + "`mcp`" + ` command provides Model Context Protocol (MCP) server functionality,
allowing AI assistants like Claude to interact with Speakeasy tooling.

## Usage with Claude Code

Add this to your Claude Code MCP configuration:

` + "```json" + `
{
  "mcpServers": {
    "speakeasy": {
      "command": "speakeasy",
      "args": ["mcp", "serve"]
    }
  }
}
` + "```" + `
`

var MCPCmd = &model.CommandGroup{
	Usage:    "mcp",
	Short:    "Model Context Protocol (MCP) server for AI assistant integration",
	Long:     utils.RenderMarkdown(mcpLong),
	Commands: []model.Command{serveCmd},
}

type serveFlags struct {
	Transport string `json:"transport"`
}

var serveCmd = &model.ExecutableCommand[serveFlags]{
	Usage: "serve",
	Short: "Start the MCP server",
	Long:  "Starts an MCP server that exposes Speakeasy CLI functionality to AI assistants.",
	Run:   runServe,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:         "transport",
			Shorthand:    "t",
			Description:  "Transport type (stdio or http)",
			DefaultValue: "stdio",
		},
	},
}

// JSON-RPC types
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   *jsonRPCError `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// MCP protocol types

type initializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    serverCapabilities `json:"capabilities"`
	ServerInfo      serverInfo         `json:"serverInfo"`
}

type serverCapabilities struct {
	Tools *toolsCapability `json:"tools,omitempty"`
}

type toolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type toolsListResult struct {
	Tools []tool `json:"tools"`
}

type callToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type callToolResult struct {
	Content []textContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func runServe(ctx context.Context, flags serveFlags) error {
	if flags.Transport != "stdio" {
		return fmt.Errorf("only stdio transport is currently supported")
	}

	server := &mcpServer{
		stdin:  os.Stdin,
		stdout: os.Stdout,
	}

	return server.run(ctx)
}

type mcpServer struct {
	stdin  io.Reader
	stdout io.Writer
}

func (s *mcpServer) run(ctx context.Context) error {
	decoder := json.NewDecoder(s.stdin)
	encoder := json.NewEncoder(s.stdout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var req jsonRPCRequest
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				return nil
			}
			continue
		}

		resp := s.handleRequest(ctx, &req)
		if resp != nil {
			if err := encoder.Encode(resp); err != nil {
				return fmt.Errorf("failed to encode response: %w", err)
			}
		}
	}
}

func (s *mcpServer) handleRequest(ctx context.Context, req *jsonRPCRequest) *jsonRPCResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "initialized":
		// Notification, no response needed
		return nil
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	case "ping":
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{},
		}
	default:
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonRPCError{
				Code:    -32601,
				Message: fmt.Sprintf("Method not found: %s", req.Method),
			},
		}
	}
}

func (s *mcpServer) handleInitialize(req *jsonRPCRequest) *jsonRPCResponse {
	result := initializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: serverCapabilities{
			Tools: &toolsCapability{},
		},
		ServerInfo: serverInfo{
			Name:    "speakeasy-mcp",
			Version: "1.0.0",
		},
	}

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func (s *mcpServer) handleToolsList(req *jsonRPCRequest) *jsonRPCResponse {
	tools := getTools()

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  toolsListResult{Tools: tools},
	}
}

func (s *mcpServer) handleToolsCall(ctx context.Context, req *jsonRPCRequest) *jsonRPCResponse {
	var params callToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonRPCError{
				Code:    -32602,
				Message: fmt.Sprintf("Invalid params: %s", err.Error()),
			},
		}
	}

	result, isError := executeTool(ctx, params.Name, params.Arguments)

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: callToolResult{
			Content: []textContent{{Type: "text", Text: result}},
			IsError: isError,
		},
	}
}
