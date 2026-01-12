# Speakeasy Plugin for Claude Code

Generate production-ready SDKs from OpenAPI specifications with guidance for the Speakeasy CLI.

## Installation

```bash
claude /install github:speakeasy-api/speakeasy/plugins/claude-code
```

## What This Provides

This plugin provides **skills** - usage guidance and decision frameworks for Speakeasy CLI operations:

### Core Commands
| Skill | Triggered By |
|-------|--------------|
| `start-new-sdk-project` | User wants to generate SDK, no .speakeasy/ exists |
| `regenerate-sdk` | User wants to regenerate, workflow.yaml exists |
| `validate-openapi-spec` | User wants to validate their spec |
| `get-ai-suggestions` | User wants AI improvements for their spec |
| `check-workspace-status` | User wants to see current configuration |

### Overlay & Merge
| Skill | Triggered By |
|-------|--------------|
| `create-openapi-overlay` | User wants to modify spec without changing original |
| `apply-openapi-overlay` | User wants to apply an overlay |
| `merge-openapi-specs` | User has multiple specs to combine |

### Troubleshooting
| Skill | Triggered By |
|-------|--------------|
| `diagnose-generation-failure` | "Step Failed: Workflow" in output |
| `fix-validation-errors-with-overlays` | Lint errors that are cosmetic |
| `improve-operation-ids` | SDK methods have auto-generated names |

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

# Regenerate after changes
speakeasy run
```

### Overlay Over Modify

Never modify the source OpenAPI spec directly. Use overlays to:
- Rename operations
- Add descriptions
- Configure SDK behavior

### Don't Auto-Fix Everything

When encountering spec issues:
- **Small issues** (naming, descriptions) → Fix with overlays
- **Structural issues** (invalid refs, missing schemas) → Ask the user
- **Design issues** (auth, API structure) → Produce strategy document

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
