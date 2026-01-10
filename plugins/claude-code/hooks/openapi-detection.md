# OpenAPI File Detection Hook

Automatically detect when the user is working with OpenAPI files and offer relevant assistance.

## Trigger

This hook runs on `PreToolUse` when file read/edit operations target OpenAPI specifications.

## Detection Patterns

Identify OpenAPI files by:

### Filename Patterns
- `openapi.yaml`, `openapi.json`
- `swagger.yaml`, `swagger.json`
- `api.yaml`, `api.json`
- `*-openapi.yaml`, `*-spec.yaml`
- Files in `specs/`, `api/`, `openapi/` directories

### Content Patterns
When reading YAML/JSON files, check for OpenAPI indicators:
```yaml
openapi: "3.0.0"  # or 3.1.0, etc.
# or
swagger: "2.0"
```

## Behavior

### When OpenAPI File Is Detected

1. **Activate OpenAPI best practices skill** silently in the background

2. **On first detection in session**, briefly note:
   ```
   💡 OpenAPI spec detected. Speakeasy commands available:
   /speakeasy-lint, /speakeasy-suggest, /speakeasy-generate
   ```

3. **When editing OpenAPI files**, be mindful of:
   - Maintaining valid YAML/JSON structure
   - Preserving existing operationIds
   - Not breaking schema references
   - Suggesting improvements inline when relevant

### When Editing OpenAPI Files

Provide contextual assistance:

```
User: Add a new endpoint for creating orders