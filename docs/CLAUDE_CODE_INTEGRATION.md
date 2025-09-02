# Claude Code Integration Guide

This document outlines enhancements made to the Speakeasy CLI to improve compatibility with Claude Code and other automation tools.

## Mise Integration

The Speakeasy CLI now includes mise configuration for consistent tooling management:

### Setup
```bash
# Install mise if not already installed
curl https://mise.run | sh

# Install tools defined in .mise.toml
mise install
```

### Available Tasks
- `mise run build` - Build the Speakeasy CLI
- `mise run test` - Run the test suite  
- `mise run lint` - Run linters
- `mise run install-dev` - Install development version locally
- `mise run speakeasy -- <args>` - Run speakeasy commands with consistent environment

### Example Usage
```bash
# Build and run locally
mise run build
mise run speakeasy -- generate sdk

# Run with specific arguments
mise run speakeasy -- run --help
```

## JSON Output Support

### Current Status
The Speakeasy CLI uses structured logging and has JSON support in several areas:

- **Internal JSON handling**: Commands use JSON for internal flag parsing and data serialization
- **Workflow files**: `.speakeasy/workflow.yaml` and `.speakeasy/gen.lock` use structured formats
- **Configuration**: Structured configuration via YAML/JSON formats

### Recommendations for Claude Code

When using the Speakeasy CLI with Claude Code, consider these patterns:

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
sources:
  - location: ./openapi.yaml
targets:
  - typescript
  - python
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

## Best Practices for Automation

### 1. Use Workflow Files
Always use `.speakeasy/workflow.yaml` for consistent configuration:
```yaml
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

### 2. Error Handling
Check exit codes and parse error output:
```bash
output=$(speakeasy generate sdk 2>&1)
exit_code=$?
if [ $exit_code -ne 0 ]; then
    echo "Error: $output"
    exit 1
fi
```

### 3. Version Pinning
Pin CLI versions in CI/CD:
```yaml
# In workflow.yaml
speakeasyVersion: "1.300.0"  # Pin to specific version
```

### 4. Environment Variables
Use environment variables for configuration:
```bash
export SPEAKEASY_API_KEY="your-key"
export SPEAKEASY_WORKSPACE_ID="your-workspace"
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

## Examples

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

## Integration Testing

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

This integration guide ensures the Speakeasy CLI works smoothly with Claude Code and other automation tools while maintaining its existing functionality and user experience.