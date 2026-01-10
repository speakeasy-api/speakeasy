# Speakeasy Plugin for Claude Code

Generate production-ready SDKs from OpenAPI specifications directly within Claude Code.

## Features

- **SDK Generation**: Generate type-safe SDKs in 10+ languages (TypeScript, Python, Go, Java, C#, PHP, Ruby, Swift, Kotlin, Terraform)
- **OpenAPI Validation**: Lint and validate your API specifications
- **AI-Powered Suggestions**: Get intelligent recommendations to improve your API design
- **Workflow Automation**: Run complete SDK generation pipelines
- **Best Practices**: Built-in guidance for OpenAPI and SDK design patterns

## Installation

### From Official Marketplace

```bash
claude /install speakeasy
```

### From GitHub

```bash
claude /install github:speakeasy-api/speakeasy/plugins/claude-code
```

### Prerequisites

The Speakeasy CLI must be installed on your system:

```bash
# macOS
brew install speakeasy-api/homebrew-tap/speakeasy

# Other platforms
curl -fsSL https://raw.githubusercontent.com/speakeasy-api/speakeasy/main/install.sh | sh
```

## Manual MCP Server Setup

If you want to use the MCP server directly without the full plugin, add this to your Claude Code MCP configuration:

```json
{
  "mcpServers": {
    "speakeasy": {
      "command": "speakeasy",
      "args": ["mcp", "serve"]
    }
  }
}
```

Or use the Claude CLI:

```bash
claude mcp add speakeasy -- speakeasy mcp serve
```

## Commands

| Command | Description |
|---------|-------------|
| `/speakeasy-generate` | Generate an SDK from an OpenAPI spec |
| `/speakeasy-lint` | Validate an OpenAPI specification |
| `/speakeasy-suggest` | Get AI-powered improvement suggestions |
| `/speakeasy-run` | Execute a Speakeasy workflow |
| `/speakeasy-quickstart` | Initialize a new Speakeasy project |

## Quick Start

1. **Install the plugin**:
   ```bash
   claude /install speakeasy
   ```

2. **Generate your first SDK**:
   ```
   /speakeasy-generate ./openapi.yaml typescript ./sdk
   ```

3. **Or set up a workflow**:
   ```
   /speakeasy-quickstart ./openapi.yaml python
   /speakeasy-run
   ```

## MCP Tools

The plugin exposes these tools via the native MCP server (`speakeasy mcp serve`):

| Tool | Description |
|------|-------------|
| `speakeasy_generate` | Generate SDK from OpenAPI spec |
| `speakeasy_lint` | Validate OpenAPI specification |
| `speakeasy_suggest` | Get improvement suggestions |
| `speakeasy_run` | Execute workflow |
| `speakeasy_quickstart` | Initialize project |
| `speakeasy_overlay_create` | Create an overlay template |
| `speakeasy_overlay_apply` | Apply an overlay |
| `speakeasy_status` | Check workspace status |
| `speakeasy_merge` | Merge multiple specs |

## Agents

### SDK Generator Agent
Handles end-to-end SDK generation workflows, from validation to generation to documentation.

### OpenAPI Improver Agent
Analyzes your OpenAPI spec and suggests improvements for better SDK quality.

## Configuration

Configure the plugin in your Claude Code settings:

```json
{
  "speakeasy.defaultTarget": "typescript",
  "speakeasy.autoLint": true,
  "speakeasy.workflowPath": ".speakeasy/workflow.yaml"
}
```

## Examples

### Generate a TypeScript SDK

```
User: Generate a TypeScript SDK from my API spec

Claude: I'll generate a TypeScript SDK from your OpenAPI spec at ./api/openapi.yaml...

✅ SDK generated at ./sdk/typescript/
- 45 API operations
- 23 model types
- Full TypeScript types included

To use:
npm install ./sdk/typescript
```

### Improve Your OpenAPI Spec

```
User: My SDK methods have ugly names, can you help?

Claude: I'll analyze your spec and suggest improvements...

Found 12 operations that could have better names:
- get_api_v1_users_list → listUsers
- post_api_v1_users_create → createUser
...

Would you like me to create an overlay with these improvements?
```

### Multi-Language Generation

```
User: I need SDKs for TypeScript, Python, and Go

Claude: I'll set up a multi-target workflow...

Created .speakeasy/workflow.yaml with 3 targets.
Running generation for all targets...

✅ Generated:
- TypeScript SDK → ./sdks/typescript
- Python SDK → ./sdks/python
- Go SDK → ./sdks/go
```

## Supported Languages

| Language | Target Name |
|----------|-------------|
| TypeScript | `typescript` |
| Python | `python` |
| Go | `go` |
| Java | `java` |
| C# | `csharp` |
| PHP | `php` |
| Ruby | `ruby` |
| Swift | `swift` |
| Kotlin | `kotlin` |
| Terraform | `terraform` |
| MCP Server | `mcp-typescript` |

## Resources

- [Speakeasy Documentation](https://speakeasy.com/docs)
- [OpenAPI Best Practices](https://speakeasy.com/docs/openapi)
- [SDK Customization](https://speakeasy.com/docs/customize-sdks)
- [GitHub](https://github.com/speakeasy-api/speakeasy)

## Support

- [GitHub Issues](https://github.com/speakeasy-api/speakeasy/issues)
- [Discord Community](https://discord.gg/speakeasy)
- [Email Support](mailto:support@speakeasy.com)

## License

Apache-2.0
