---
name: configure-authentication
description: Use when setting up Speakeasy authentication, configuring API keys, or troubleshooting auth errors in non-interactive/CI environments
---

# configure-authentication

Configure Speakeasy CLI authentication for non-interactive environments like CI/CD pipelines and AI agents.

## Environment Variable (Recommended)

Set `SPEAKEASY_API_KEY` for non-interactive authentication:

```bash
export SPEAKEASY_API_KEY="<your-api-key>"
```

This takes precedence over config file settings.

## Config File

Alternatively, create `~/.speakeasy/config.yaml`:

```yaml
speakeasy_api_key: "<your-api-key>"
speakeasy_workspace_id: "<workspace-id>"
```

## Getting Your API Key

1. Go to [Speakeasy Dashboard](https://app.speakeasy.com)
2. Navigate to **Settings** > **API Keys**
3. Create or copy your API key

## CI/CD Examples

### GitHub Actions

```yaml
env:
  SPEAKEASY_API_KEY: ${{ secrets.SPEAKEASY_API_KEY }}
```

### Docker

```bash
docker run -e SPEAKEASY_API_KEY="$SPEAKEASY_API_KEY" ...
```

### Shell Script

```bash
#!/bin/bash
export SPEAKEASY_API_KEY="${SPEAKEASY_API_KEY:?SPEAKEASY_API_KEY must be set}"
speakeasy run
```

## Verifying Authentication

```bash
# Check if authenticated (returns workspace info)
speakeasy auth login --check 2>/dev/null && echo "Authenticated" || echo "Not authenticated"
```

## Common Auth Errors

| Error | Cause | Solution |
|-------|-------|----------|
| `unauthorized` | Missing or invalid API key | Set `SPEAKEASY_API_KEY` env var |
| `workspace not found` | Wrong workspace ID | Check `speakeasy_workspace_id` in config |
| `TTY required` | Interactive login attempted | Use env var instead of `speakeasy auth login` |

## Multiple Workspaces

For multiple workspaces, use workspace-specific keys in config:

```yaml
workspace_api_keys:
  org-slug@workspace-slug: "<api-key-1>"
  other-org@other-workspace: "<api-key-2>"
```
