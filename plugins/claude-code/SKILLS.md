# Speakeasy Skills for Claude Code

This file provides troubleshooting guidance for common Speakeasy CLI issues. Each skill is triggered by specific error patterns you may encounter.

## Skills

- diagnose-openapi-spec-issues-related-to-generation: speakeasy quickstart / run failed with and output includes Step Failed: Workflow
- fix-validation-errors-with-overlays: speakeasy lint output shows validation errors that can be fixed without changing the source spec
- understand-speakeasy-workflow: need to understand how .speakeasy/workflow.yaml works or troubleshoot workflow configuration
- improve-operation-ids: speakeasy suggest operation-ids or SDK methods have auto-generated names like GetApiV1Users

---

## diagnose-openapi-spec-issues-related-to-generation

**Triggered by:** `Step Failed: Workflow` in speakeasy output

When SDK generation fails, you need to determine the root cause and appropriate fix strategy.

### Diagnosis Steps

1. **Run `speakeasy lint openapi -s <spec-path>`** to get detailed validation errors
2. **Categorize the issues:**
   - **Fixable with overlays:** Missing descriptions, poor operation IDs, missing tags
   - **Requires spec changes:** Invalid schema structure, missing required fields, circular references
   - **Requires author input:** Ambiguous API design, unclear response types, authentication questions

### Decision Framework

| Issue Type | Fix Strategy | Example |
|------------|--------------|---------|
| Missing operationId | Overlay with x-speakeasy-name-override | `get /users` → `listUsers` |
| Missing description | Overlay to add descriptions | Add summary to endpoints |
| Invalid $ref | **Ask user** - spec is broken | `$ref: '#/components/schemas/Missing'` |
| Circular reference | **Ask user** - design decision needed | A references B references A |
| Missing security scheme | **Ask user** - auth design needed | No auth defined but endpoints need it |
| Unsupported OpenAPI feature | Overlay to work around | allOf with discriminator edge cases |

### What NOT to Do

- **Do NOT** disable lint rules to make errors go away
- **Do NOT** try to fix every issue one-by-one with overlays
- **Do NOT** modify the source OpenAPI spec directly unless explicitly asked
- **Do NOT** assume you can fix structural spec problems

### Output Strategy Document

When issues are complex, produce a strategy document for the user:

```markdown
## OpenAPI Spec Analysis

### Blocking Issues (require author input)
- Issue 1: [description]
- Issue 2: [description]

### Fixable Issues (can use overlays)
- Issue 1: [description] → [proposed fix]
- Issue 2: [description] → [proposed fix]

### Recommended Approach
[Your recommendation on how to proceed]
```

---

## fix-validation-errors-with-overlays

**Triggered by:** Validation errors from `speakeasy lint` that are cosmetic or naming-related

Overlays let you modify an OpenAPI spec without changing the original file. Use them for non-structural improvements.

### When to Use Overlays

**Good candidates for overlays:**
- Renaming operations (x-speakeasy-name-override)
- Adding missing descriptions/summaries
- Adding x-speakeasy-group for SDK organization
- Marking deprecated endpoints
- Adding x-speakeasy-retries configuration

**NOT candidates for overlays:**
- Fixing invalid JSON/YAML syntax
- Adding missing required schema properties
- Fixing broken $ref references
- Restructuring the API design

### Creating an Overlay

```bash
# Generate overlay template
speakeasy overlay create -s openapi.yaml -o overlay.yaml
```

### Example Overlay for Operation Naming

```yaml
overlay: 1.0.0
info:
  title: Improve SDK method names
  version: 1.0.0
actions:
  - target: "$.paths['/users'].get"
    update:
      x-speakeasy-name-override: listUsers
  - target: "$.paths['/users'].post"
    update:
      x-speakeasy-name-override: createUser
  - target: "$.paths['/users/{id}'].get"
    update:
      x-speakeasy-name-override: getUser
```

### Applying Overlays

Add to workflow.yaml:
```yaml
sources:
  my-api:
    inputs:
      - location: ./openapi.yaml
    overlays:
      - location: ./overlay.yaml
```

Then run: `speakeasy run`

---

## understand-speakeasy-workflow

**Triggered by:** Questions about workflow.yaml or `speakeasy run` behavior

The `.speakeasy/workflow.yaml` file defines your SDK generation pipeline.

### Workflow Structure

```yaml
workflowVersion: 1.0.0
speakeasyVersion: latest
sources:
  my-api:                          # Source name (referenced by targets)
    inputs:
      - location: ./openapi.yaml   # Path or URL to OpenAPI spec
    overlays:
      - location: ./overlay.yaml   # Optional overlays to apply
targets:
  typescript-sdk:                  # Target name
    target: typescript             # Language: typescript, python, go, java, etc.
    source: my-api                 # Which source to use
    output: ./sdks/typescript      # Output directory
```

### Common Commands

```bash
# Check workspace status
speakeasy status

# Run all targets
speakeasy run

# Run specific target
speakeasy run -t typescript-sdk

# Initialize new project
speakeasy quickstart -s openapi.yaml -t typescript
```

### Multi-Target Setup

```yaml
targets:
  typescript-sdk:
    target: typescript
    source: my-api
    output: ./sdks/typescript
  python-sdk:
    target: python
    source: my-api
    output: ./sdks/python
  go-sdk:
    target: go
    source: my-api
    output: ./sdks/go
```

### Troubleshooting Workflow Issues

| Symptom | Cause | Fix |
|---------|-------|-----|
| "Source not found" | Typo in source name | Check source name matches between sources and targets |
| "File not found" | Wrong path to spec | Use absolute path or path relative to workflow.yaml location |
| "Invalid spec" | Spec has errors | Run `speakeasy lint openapi -s <spec>` first |

---

## improve-operation-ids

**Triggered by:** SDK methods have auto-generated names or user asks about operation naming

Good operation IDs create intuitive SDK method names.

### Check Current State

```bash
speakeasy suggest operation-ids -s openapi.yaml
```

This shows what operation IDs would be suggested.

### Naming Conventions

| HTTP Method | Good Name | Bad Name |
|-------------|-----------|----------|
| GET /users | listUsers | getApiV1UsersList |
| GET /users/{id} | getUser | getApiV1UsersById |
| POST /users | createUser | postApiV1Users |
| PUT /users/{id} | updateUser | putApiV1UsersById |
| DELETE /users/{id} | deleteUser | deleteApiV1UsersById |

### Apply Suggestions

```bash
# Generate overlay with suggested operation IDs
speakeasy suggest operation-ids -s openapi.yaml -o operation-ids-overlay.yaml
```

Then add to workflow.yaml and run `speakeasy run`.

### Manual Override with Overlay

If you want custom names different from suggestions:

```yaml
overlay: 1.0.0
info:
  title: Custom operation names
  version: 1.0.0
actions:
  - target: "$.paths['/api/v1/users'].get"
    update:
      operationId: listAllUsers
      x-speakeasy-name-override: listAllUsers
```
