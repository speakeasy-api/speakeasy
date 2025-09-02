# Speakeasy CLI - Project Context & Claude Code Integration

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

---

# Claude Code Integration

## Build and Development

### Standard Go Commands
```bash
# Build the CLI
go build -o speakeasy .

# Run tests
go test ./...

# Run linters
golangci-lint run
```

### Mise Integration

The Speakeasy CLI includes mise configuration for consistent tooling management:

#### Setup
```bash
# Install mise if not already installed
curl https://mise.run | sh

# Install tools defined in .mise.toml
mise install
```

#### Available Tasks
- `mise run build` - Build the Speakeasy CLI
- `mise run test` - Run the test suite  
- `mise run lint` - Run linters
- `mise run install-dev` - Install development version locally
- `mise run speakeasy -- <args>` - Run speakeasy commands with consistent environment

#### Example Usage
```bash
# Build and run locally
mise run build
mise run speakeasy -- generate sdk

# Run with specific arguments
mise run speakeasy -- run --help
```

## Automation Support

### Current JSON/Structured Output
The Speakeasy CLI uses structured logging and has JSON support in several areas:

- **Internal JSON handling**: Commands use JSON for internal flag parsing and data serialization
- **Workflow files**: `.speakeasy/workflow.yaml` and `.speakeasy/gen.lock` use structured formats
- **Configuration**: Structured configuration via YAML/JSON formats

### Claude Code Best Practices

#### 1. Exit Code Handling
```bash
# Check exit codes for automation
speakeasy generate sdk
if [ $? -eq 0 ]; then
    echo "Generation successful"
else
    echo "Generation failed"
fi
```

#### 2. Structured Configuration
Use workflow files for consistent, repeatable operations:
```yaml
# .speakeasy/workflow.yaml
version: 1.0.0
speakeasyVersion: latest
sources:
  my-source:
    location: ./openapi.yaml
targets:
  typescript:
    target: typescript
    source: my-source
```

#### 3. Non-Interactive Mode
The CLI automatically detects non-interactive environments and adjusts behavior:
- Disables interactive prompts
- Uses structured output formats
- Provides clear error messages

#### 4. File Output Parsing
Many commands generate structured files that can be parsed:
```bash
# Generate and parse results
speakeasy generate sdk
cat .speakeasy/gen.lock | jq '.targets'
```

#### 5. Error Handling
Check exit codes and parse error output:
```bash
output=$(speakeasy generate sdk 2>&1)
exit_code=$?
if [ $exit_code -ne 0 ]; then
    echo "Error: $output"
    exit 1
fi
```

#### 6. Version Pinning
Pin CLI versions in CI/CD:
```yaml
# In workflow.yaml
speakeasyVersion: "1.300.0"  # Pin to specific version
```

#### 7. Environment Variables
Use environment variables for configuration:
```bash
export SPEAKEASY_API_KEY="your-key"
export SPEAKEASY_WORKSPACE_ID="your-workspace"
```

## Integration Examples

### Basic Workflow with Claude Code
```bash
# 1. Initialize project
speakeasy quickstart

# 2. Generate SDK
mise run speakeasy -- generate sdk --target typescript

# 3. Check results
if [ -f "typescript-sdk/package.json" ]; then
    echo "TypeScript SDK generated successfully"
fi
```

### Advanced Automation
```bash
#!/bin/bash
set -e

# Install dependencies via mise
mise install

# Build custom version
mise run build

# Generate multiple targets
for target in typescript python go; do
    echo "Generating $target SDK..."
    mise run speakeasy -- generate sdk --target $target
    
    if [ $? -eq 0 ]; then
        echo "✓ $target SDK generated"
    else
        echo "✗ $target SDK failed"
        exit 1
    fi
done

echo "All SDKs generated successfully"
```

### Integration Testing
Test CLI behavior in non-interactive environments:
```bash
# Test non-interactive detection
echo "test" | speakeasy generate sdk

# Test exit codes
speakeasy validate openapi.yaml
echo "Exit code: $?"

# Test structured output parsing
speakeasy run | grep -E "(success|error|warning)"
```

## Future Enhancements

### Proposed JSON Output Flags
Consider adding these flags to major commands:
- `--json` - Output results in JSON format
- `--quiet` - Suppress non-essential output
- `--format=json|yaml|table` - Specify output format

### Enhanced Error Reporting
- Structured error objects with error codes
- Machine-readable error categories
- Detailed context information

### Batch Operations
- Support for processing multiple specs in one command
- Bulk operations with structured results
- Progress reporting in structured format

## Testing Philosophy

- Commands should work in both interactive and non-interactive modes
- Clear error messages for debugging
- Reliable exit codes for automation
- Integration tests for end-to-end workflows

## Memory Context for Claude

When working on this codebase:
- Always consider both interactive and non-interactive usage
- Follow the established command patterns in `internal/model/`
- Use structured logging for consistency
- Test automation scenarios
- Consider impact on existing workflows and configurations
- The CLI integrates with GitHub Actions, CI/CD systems, and package managers
- Workflow files are central to the system's operation and repeatability