# Speakeasy Plugin for Claude Code

Generate production-ready SDKs from OpenAPI specifications with guidance for the Speakeasy CLI.

## Installation

```bash
claude /install github:speakeasy-api/speakeasy/plugins/claude-code
```

## What This Provides

This plugin provides **skills** - usage guidance and decision frameworks for Speakeasy CLI operations:

| Skill | Use When... |
|-------|-------------|
| `start-new-sdk-project` | You have an OpenAPI spec and want to generate an SDK |
| `regenerate-sdk` | Your spec changed and you need to regenerate |
| `validate-openapi-spec` | Checking if spec is valid, running `speakeasy lint` |
| `get-ai-suggestions` | SDK method names are ugly, wanting to improve operation IDs |
| `check-workspace-status` | Asking what targets/sources are configured |
| `create-openapi-overlay` | Need to customize SDK without editing source spec |
| `apply-openapi-overlay` | Applying an overlay file to a spec |
| `merge-openapi-specs` | Combining multiple OpenAPI specs |
| `diagnose-generation-failure` | SDK generation failed, seeing "Step Failed: Workflow" |
| `fix-validation-errors-with-overlays` | Have lint errors but can't modify source spec |
| `improve-operation-ids` | SDK methods have names like GetApiV1Users |

## Philosophy

Claude Code already calls CLIs reliably. This plugin doesn't wrap CLI commands - instead it provides:

1. **Usage guidance** - How to use each command effectively
2. **Decision frameworks** - When to use overlays vs ask the user
3. **Troubleshooting** - How to diagnose and fix common issues

## Key Principles

### Use the Workflow

```bash
# Initialize project
speakeasy quickstart -s openapi.yaml -t typescript

# Regenerate after changes (use --output console for AI-friendly output)
speakeasy run --output console
```

### Overlay Over Modify

Never modify the source OpenAPI spec directly. Use overlays to:
- Rename operations
- Add descriptions
- Configure SDK behavior
- Make portable patches across spec versions

### Don't Auto-Fix Everything

When encountering spec issues:
- **Small issues** (naming, descriptions) → Fix with overlays
- **Structural issues** (invalid refs, missing schemas) → Ask the user
- **Design issues** (auth, API structure) → Produce strategy document

## Plugin Structure

```
plugins/claude-code/
├── .claude-plugin/
│   └── plugin.json       # Plugin metadata
├── skills/               # Individual skill files
│   ├── start-new-sdk-project.md
│   ├── regenerate-sdk.md
│   ├── validate-openapi-spec.md
│   └── ...
├── marketplace.json      # Marketplace metadata
└── README.md
```

## Prerequisites

Install Speakeasy CLI:

```bash
# macOS
brew install speakeasy-api/homebrew-tap/speakeasy

# Other platforms
curl -fsSL https://raw.githubusercontent.com/speakeasy-api/speakeasy/main/install.sh | sh
```

## Resources

- [Speakeasy Documentation](https://speakeasy.com/docs)
- [OpenAPI Best Practices](https://speakeasy.com/docs/openapi)
- [SDK Customization](https://speakeasy.com/docs/customize-sdks)

## License

Apache-2.0
