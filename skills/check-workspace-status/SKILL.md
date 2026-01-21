---
name: check-workspace-status
description: Use when asking what targets/sources are configured, or wanting to see current Speakeasy setup
---

# check-workspace-status

Use `speakeasy status` to view workspace state.

## Prerequisites

For non-interactive environments (CI/CD, AI agents), set:
```bash
export SPEAKEASY_API_KEY="<your-api-key>"
```
See `configure-authentication` skill for details.

## Command

```bash
# Default visual output (requires TTY)
speakeasy status

# Plain text for non-TTY environments (CI/CD, AI agents)
speakeasy status --output console

# Structured JSON for automation and parsing
speakeasy status --output json
```

## Output Modes

| Mode | Flag | Use Case |
|------|------|----------|
| summary | `--output summary` (default) | Interactive terminals with TTY |
| console | `--output console` | CI/CD, AI agents, non-interactive |
| json | `--output json` | Automation, scripting, programmatic access |

## What It Shows

- Workspace name and account type
- Published targets (with version, URLs, last publish/generate info)
- Configured targets (unpublished, with repository URLs)
- Unconfigured targets
- Any generation failures or upgrade recommendations

## Use Cases

- Verify setup before running generation
- Debug workflow configuration issues
- Check what targets are configured
- Automate status checks in CI/CD pipelines
- Parse workspace state programmatically with `--output json`
