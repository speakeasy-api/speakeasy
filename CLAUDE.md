# Speakeasy CLI Project Context

## Project Overview

The Speakeasy CLI is a command-line tool for generating SDKs, Terraform providers, and other developer tools from OpenAPI specifications. It's written in Go and uses the Cobra framework for CLI structure.

## Key Architecture

- **Command Structure**: Uses Cobra commands with a modern `ExecutableCommand` pattern in `internal/model/command.go`
- **Code Generation**: Core functionality generates type-safe SDKs in 10+ programming languages
- **Workflow System**: Uses `.speakeasy/workflow.yaml` files for configuration and repeatability
- **Authentication**: Integrates with Speakeasy platform API for workspace management
- **Interactive Mode**: Supports both interactive and non-interactive (CI/CD) usage

## Important Directories

```
cmd/                    # CLI command implementations
internal/model/         # Command framework and patterns
internal/auth/         # Authentication and workspace management
internal/run/          # Core workflow execution
prompts/               # Interactive prompt handling
pkg/                   # Public packages
integration/           # Integration tests
```

## Development Patterns

### Command Creation
- Use `ExecutableCommand[F]` pattern from `internal/model/command.go`
- Flags are defined as structs with JSON tags
- Commands support both interactive and non-interactive modes

### Error Handling
- Use `internal/log` for structured logging
- Commands should have clear exit codes for automation
- Support both human-readable and machine-parseable errors

### Testing
- Integration tests in `integration/` directory
- Use `internal/testutils` for test utilities
- Commands should work in non-interactive environments

## Code Style Guidelines

- **Go Conventions**: Follow standard Go idioms and conventions
- **Cobra Patterns**: Use the established command patterns
- **Logging**: Use structured logging with context
- **Error Messages**: Provide clear, actionable error messages

## Build and Development

```bash
# Build the CLI
go build -o speakeasy .

# Run tests
go test ./...

# Run linters
golangci-lint run

# Using mise (if available)
mise run build
mise run test
mise run lint
```

## Key Features

1. **SDK Generation**: Primary feature - generates SDKs from OpenAPI specs
2. **Workflow Management**: Manages generation workflows with version tracking
3. **Platform Integration**: Connects to Speakeasy platform for enhanced features
4. **Multi-Language Support**: Supports TypeScript, Python, Go, Java, C#, PHP, Ruby, Unity
5. **Automation-Friendly**: Works well in CI/CD environments

## Common Tasks

### Adding a New Command
1. Create command struct using `ExecutableCommand[F]` pattern
2. Define flags struct with JSON tags
3. Implement run logic with proper error handling
4. Add to command hierarchy in appropriate parent

### Modifying Generation Logic
- Core generation logic is in `internal/run/`
- Workflow handling in `internal/run/workflow.go`
- Source and target management in respective files

### Authentication Changes
- Authentication logic in `internal/auth/`
- Workspace management and API key handling
- Session management for interactive vs non-interactive modes

## Integration Notes

The CLI is designed to work well with:
- **GitHub Actions**: Automated SDK generation workflows
- **Claude Code**: Enhanced automation support (see docs/CLAUDE_CODE_INTEGRATION.md)
- **CI/CD Systems**: Non-interactive mode with clear exit codes
- **Package Managers**: Automated publishing to npm, PyPI, etc.

## Testing Philosophy

- Commands should work in both interactive and non-interactive modes
- Clear error messages for debugging
- Reliable exit codes for automation
- Integration tests for end-to-end workflows

## Memory Context

When working on this codebase:
- Always consider both interactive and non-interactive usage
- Follow the established command patterns in `internal/model/`
- Use structured logging for consistency
- Test automation scenarios
- Consider impact on existing workflows and configurations