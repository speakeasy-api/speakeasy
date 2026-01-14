---
description: Use when working with Speakeasy CLI, generating SDKs from OpenAPI specs, troubleshooting SDK generation failures, or asking about speakeasy commands
---

# Speakeasy Skills for Claude Code

## Skills

| name | description |
| --- | --- |
| start-new-sdk-project | Use when you have an OpenAPI spec and want to generate an SDK, or asking "how do I get started with Speakeasy" |
| regenerate-sdk | Use when your spec changed and you need to regenerate the SDK, or running `speakeasy run` |
| validate-openapi-spec | Use when checking if an OpenAPI spec is valid, looking for errors, or running `speakeasy lint` |
| get-ai-suggestions | Use when SDK method names are ugly, wanting to improve operation IDs, or asking "how can I improve my spec" |
| check-workspace-status | Use when asking what targets/sources are configured, or wanting to see current Speakeasy setup |
| create-openapi-overlay | Use when you need to customize SDK generation without editing the source spec, or can't modify the original OpenAPI file |
| apply-openapi-overlay | Use when applying an overlay file to a spec |
| merge-openapi-specs | Use when combining multiple OpenAPI specs, or have microservices with separate spec files |
| diagnose-generation-failure | Use when SDK generation failed, seeing "Step Failed: Workflow", or `speakeasy run` errors |
| fix-validation-errors-with-overlays | Use when you have lint errors but can't modify the source spec, or need to add missing descriptions/tags via overlay |
| improve-operation-ids | Use when SDK methods have auto-generated names like GetApiV1Users, or wanting `sdk.users.list()` style naming |

---

## start-new-sdk-project

**Triggered by:** User wants to generate an SDK and no `.speakeasy/` directory exists

Use `speakeasy quickstart` to initialize a new project with a workflow configuration.

### Command

```bash
speakeasy quickstart -s <path-to-openapi-spec> -t <target-language>
```

### Supported Targets

| Language | Target Flag |
|----------|-------------|
| TypeScript | `typescript` |
| Python | `python` |
| Go | `go` |
| Java | `java` |
| C# | `csharp` |
| PHP | `php` |
| Ruby | `ruby` |
| Kotlin | `kotlin` |
| Terraform | `terraform` |

### Example

```bash
# Initialize TypeScript SDK project
speakeasy quickstart -s ./api/openapi.yaml -t typescript

# With custom output directory
speakeasy quickstart -s ./api/openapi.yaml -t python -o ./sdks/python
```

### What It Creates

```
.speakeasy/
└── workflow.yaml    # Workflow configuration
```

### Next Steps After Quickstart

1. Run `speakeasy run` to generate the SDK
2. Review the generated SDK in the output directory
3. Add more targets to workflow.yaml for multi-language support

---

## regenerate-sdk

**Triggered by:** User wants to regenerate SDK, `.speakeasy/workflow.yaml` exists

Use `speakeasy run` to execute the workflow and regenerate SDKs.

### Command

```bash
# Run all configured targets
speakeasy run

# Run specific target only
speakeasy run -t <target-name>

# Run with specific source
speakeasy run -s <source-name>
```

### When to Use

- After updating the OpenAPI spec
- After modifying workflow.yaml
- After changing overlays
- To regenerate with latest Speakeasy version

### Example Workflow

```yaml
# .speakeasy/workflow.yaml
workflowVersion: 1.0.0
speakeasyVersion: latest
sources:
  my-api:
    inputs:
      - location: ./openapi.yaml
targets:
  typescript-sdk:
    target: typescript
    source: my-api
    output: ./sdk/typescript
```

### Troubleshooting

If `speakeasy run` fails, check:
1. Is the OpenAPI spec valid? Run `speakeasy lint openapi -s <spec>`
2. Does the source path exist? Check `inputs.location` in workflow.yaml
3. Are there blocking validation errors? See `diagnose-generation-failure` skill

---

## validate-openapi-spec

**Triggered by:** User wants to validate their OpenAPI spec

Use `speakeasy lint` to check for errors and warnings.

### Command

```bash
speakeasy lint openapi -s <path-to-spec>
```

### Output Categories

| Severity | Meaning | Action |
|----------|---------|--------|
| Error | Blocks SDK generation | Must fix |
| Warning | May cause issues | Should fix |
| Hint | Best practice suggestion | Consider fixing |

### Common Validation Issues

| Issue | Solution |
|-------|----------|
| Missing operationId | Add operationId or use `speakeasy suggest operation-ids` |
| Invalid $ref | Fix the reference path |
| Missing response schema | Add response schema definitions |
| Duplicate operationId | Make operation IDs unique |

### AI-Friendly Output

For AI agents, use console output mode which provides structured, parseable output:
```bash
speakeasy run --output console
```

For commands with large outputs, pipe to `grep` or `tail` to reduce context:
```bash
speakeasy lint openapi -s ./openapi.yaml 2>&1 | grep -E "(error|warning)"
speakeasy run --output console 2>&1 | tail -50
```

---

## get-ai-suggestions

**Triggered by:** User wants to improve their OpenAPI spec with AI suggestions

Use `speakeasy suggest` for AI-powered improvements.

### Commands

```bash
# Suggest better operation IDs (method names)
speakeasy suggest operation-ids -s <spec-path>

# Suggest error type definitions
speakeasy suggest error-types -s <spec-path>

# Output suggestions as overlay file
speakeasy suggest operation-ids -s <spec-path> -o suggested-overlay.yaml
```

### Operation ID Suggestions

Transforms auto-generated names into intuitive SDK method names:
- `get_api_v1_users_list` → `listUsers`
- `post_api_v1_users_create` → `createUser`

### Error Type Suggestions

Analyzes your API and suggests structured error responses:
- Common HTTP error codes (400, 401, 404, 500)
- Custom error schemas

### Applying Suggestions

```bash
# Generate overlay with suggestions
speakeasy suggest operation-ids -s openapi.yaml -o operation-ids-overlay.yaml

# Add to workflow.yaml
sources:
  my-api:
    inputs:
      - location: ./openapi.yaml
    overlays:
      - location: ./operation-ids-overlay.yaml

# Regenerate
speakeasy run
```

---

## check-workspace-status

**Triggered by:** User wants to see current Speakeasy configuration

Use `speakeasy status` to view workspace state.

### Command

```bash
speakeasy status
```

### What It Shows

- Configured sources and their locations
- Configured targets and output directories
- Current Speakeasy version
- Any configuration issues

### Use Cases

- Verify setup before running generation
- Debug workflow configuration issues
- Check what targets are configured

---

## create-openapi-overlay

**Triggered by:** User wants to modify spec without changing the original file

Overlays let you customize an OpenAPI spec for SDK generation without modifying the source.

### Create Overlay Template

```bash
speakeasy overlay create -s <spec-path> -o <output-path>
```

### When to Use Overlays

**Overlays are great for:**
- Renaming operations (x-speakeasy-name-override)
- Adding descriptions/summaries
- Grouping operations (x-speakeasy-group)
- Adding retry configuration
- Marking endpoints as deprecated
- Adding SDK-specific extensions
- Fixing spec issues without modifying the source
- Adding new endpoints or schemas
- Making portable patches that work across spec versions

**Overlays cannot easily handle:**
- Deduplication of schemas (requires structural analysis)

### Example Overlay

```yaml
overlay: 1.0.0
info:
  title: SDK Customizations
  version: 1.0.0
actions:
  - target: "$.paths['/users'].get"
    update:
      x-speakeasy-group: users
      x-speakeasy-name-override: list
  - target: "$.paths['/users'].post"
    update:
      x-speakeasy-group: users
      x-speakeasy-name-override: create
  - target: "$.paths['/users/{id}'].get"
    update:
      x-speakeasy-group: users
      x-speakeasy-name-override: get
  - target: "$.paths['/users/{id}'].delete"
    update:
      x-speakeasy-group: users
      x-speakeasy-name-override: delete
      deprecated: true
```

This produces: `sdk.users.list()`, `sdk.users.create()`, `sdk.users.get()`, `sdk.users.delete()`

### JSONPath Targeting

| Target | Selects |
|--------|---------|
| `$.paths['/users'].get` | GET /users operation |
| `$.paths['/users/{id}'].*` | All operations on /users/{id} |
| `$.components.schemas.User` | User schema |
| `$.info` | API info object |

---

## apply-openapi-overlay

**Triggered by:** User wants to apply an overlay to a spec

### Command

```bash
speakeasy overlay apply -s <spec-path> -o <overlay-path> --out <output-path>
```

### Example

```bash
# Apply overlay and output merged spec
speakeasy overlay apply -s openapi.yaml -o my-overlay.yaml --out openapi-modified.yaml
```

### Using in Workflow

Better approach - add overlay to workflow.yaml:

```yaml
sources:
  my-api:
    inputs:
      - location: ./openapi.yaml
    overlays:
      - location: ./naming-overlay.yaml
      - location: ./grouping-overlay.yaml
```

Overlays are applied in order, so later overlays can override earlier ones.

---

## merge-openapi-specs

**Triggered by:** User has multiple OpenAPI specs to combine

Use `speakeasy merge` to combine multiple specs into one.

### Command

```bash
speakeasy merge -o <output-path> <spec1> <spec2> [spec3...]
```

### Example

```bash
# Merge two specs
speakeasy merge -o combined.yaml ./api/users.yaml ./api/orders.yaml

# Merge multiple specs
speakeasy merge -o combined.yaml ./specs/*.yaml
```

### Use Cases

- Microservices with separate specs per service
- API versioning with multiple spec files
- Combining public and internal API specs

### Conflict Resolution

When specs have conflicts:
- Later specs override earlier ones for duplicate paths
- Schema conflicts may require manual resolution
- Review merged output for correctness

---

## diagnose-generation-failure

**Triggered by:** `Step Failed: Workflow` in speakeasy output

When SDK generation fails, determine the root cause and fix strategy.

### Diagnosis Steps

1. **Run lint to get detailed errors:**
   ```bash
   speakeasy lint openapi -s <spec-path>
   ```

2. **Categorize issues:**
   - **Fixable with overlays:** Missing descriptions, poor operation IDs
   - **Requires spec fix:** Invalid schema, missing required fields
   - **Requires user input:** Design decisions, authentication setup

### Decision Framework

| Issue Type | Fix Strategy | Example |
|------------|--------------|---------|
| Missing operationId | Overlay | Use `speakeasy suggest operation-ids` |
| Missing description | Overlay | Add via overlay |
| Invalid $ref | **Ask user** | Broken reference needs spec fix |
| Circular reference | **Ask user** | Design decision needed |
| Missing security | **Ask user** | Auth design needed |

### What NOT to Do

- **Do NOT** disable lint rules to hide errors
- **Do NOT** try to fix every issue one-by-one
- **Do NOT** modify source spec without asking
- **Do NOT** assume you can fix structural problems

### Strategy Document

For complex issues, produce a document:

```markdown
## OpenAPI Spec Analysis

### Blocking Issues (require user input)
- [List issues that need human decision]

### Fixable Issues (can use overlays)
- [List issues with proposed overlay fixes]

### Recommended Approach
[Your recommendation]
```

---

## fix-validation-errors-with-overlays

**Triggered by:** Lint errors that are cosmetic or naming-related

### Overlay-Appropriate Fixes

| Issue | Overlay Solution |
|-------|------------------|
| Poor operation names | `x-speakeasy-name-override` |
| Missing descriptions | Add `summary` or `description` |
| Missing tags | Add `tags` array |
| Need operation grouping | `x-speakeasy-group` |
| Need retry config | `x-speakeasy-retries` |

### NOT Overlay-Appropriate

| Issue | Why |
|-------|-----|
| Invalid JSON/YAML | Syntax error in source |
| Missing required fields | Schema incomplete |
| Broken $ref | Source needs fixing |
| Wrong data types | API design issue |

### Quick Fix Workflow

```bash
# 1. Generate suggestions
speakeasy suggest operation-ids -s openapi.yaml -o fixes.yaml

# 2. Add to workflow
# Edit .speakeasy/workflow.yaml to include overlay

# 3. Regenerate
speakeasy run
```

---

## improve-operation-ids

**Triggered by:** SDK methods have auto-generated names

### Check Current State

```bash
speakeasy suggest operation-ids -s openapi.yaml
```

### SDK Method Naming

Speakeasy generates grouped SDK methods using `x-speakeasy-group`:

| HTTP Method | SDK Usage | Operation ID |
|-------------|-----------|--------------|
| GET (list) | `sdk.users.list()` | `users_list` |
| GET (single) | `sdk.users.get()` | `users_get` |
| POST | `sdk.users.create()` | `users_create` |
| PUT | `sdk.users.update()` | `users_update` |
| PATCH | `sdk.users.patch()` | `users_patch` |
| DELETE | `sdk.users.delete()` | `users_delete` |

Use `x-speakeasy-group: users` and `x-speakeasy-name-override: list` to achieve this grouping.

### Apply Suggestions

```bash
# Generate overlay
speakeasy suggest operation-ids -s openapi.yaml -o operation-ids.yaml

# Add to workflow and regenerate
speakeasy run
```

### Manual Override

```yaml
overlay: 1.0.0
info:
  title: Custom operation names
  version: 1.0.0
actions:
  - target: "$.paths['/api/v1/users'].get"
    update:
      x-speakeasy-group: users
      x-speakeasy-name-override: listAll
```

This produces: `sdk.users.listAll()`
