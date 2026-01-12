# Speakeasy Plugin for Claude Code

Troubleshooting guidance and best practices for SDK generation with Speakeasy.

## Installation

Copy `SKILLS.md` to your project or Claude Code configuration to provide context when working with Speakeasy.

## What This Provides

This plugin provides **skills** - troubleshooting guidance triggered by common error patterns:

| Skill | Triggered By |
|-------|--------------|
| diagnose-openapi-spec-issues-related-to-generation | `Step Failed: Workflow` in output |
| fix-validation-errors-with-overlays | Validation errors from `speakeasy lint` |
| understand-speakeasy-workflow | Questions about workflow.yaml |
| improve-operation-ids | Auto-generated SDK method names |

## Philosophy

Claude Code already calls CLIs reliably. This plugin doesn't wrap CLI commands - instead it provides:

1. **Decision frameworks** - When to use overlays vs ask the user
2. **Error pattern recognition** - What different errors mean
3. **Strategy guidance** - How to approach complex spec issues

## Key Principles

### Don't Auto-Fix Everything

When encountering spec issues:
- **Small issues** (naming, descriptions) → Fix with overlays
- **Structural issues** (invalid refs, missing schemas) → Ask the user
- **Design issues** (auth, API structure) → Produce strategy document

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
