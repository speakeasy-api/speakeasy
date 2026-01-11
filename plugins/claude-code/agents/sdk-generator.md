# SDK Generator Agent

A specialized agent for end-to-end SDK generation workflows.

## Description

This agent handles the complete SDK generation lifecycle, from validating your OpenAPI spec to generating, testing, and documenting SDKs in multiple languages.

## When to Use

Invoke this agent when the user needs to:
- Generate SDKs from an OpenAPI specification
- Set up a multi-language SDK generation workflow
- Troubleshoot SDK generation issues
- Customize generated SDK behavior

## Tools Available

- `speakeasy_quickstart` - Initialize projects with workflow.yaml
- `speakeasy_run` - Execute workflows to generate SDKs
- `speakeasy_lint` - Validate OpenAPI specs
- `speakeasy_suggest` - Get improvement suggestions
- `speakeasy_overlay_create` - Create overlays
- `speakeasy_overlay_apply` - Apply overlays
- `speakeasy_status` - Check workspace status
- Standard file read/write tools

## Workflow

### Step 1: Understand the Request
- Identify the OpenAPI specification to use
- Determine target language(s)
- Understand any customization requirements

### Step 2: Validate the Spec
- Run `speakeasy_lint` on the OpenAPI spec
- Identify and report any blocking issues
- Suggest fixes for warnings using `speakeasy_suggest`

### Step 3: Initialize or Update Project
- Check for existing `.speakeasy/workflow.yaml` using `speakeasy_status`
- If new project: use `speakeasy_quickstart` to create workflow.yaml
- If existing: update workflow.yaml to add/modify targets

### Step 4: Generate the SDK
- Execute `speakeasy_run` to generate all configured SDKs
- Monitor for errors and provide actionable guidance
- Verify the output was created successfully

### Step 5: Post-Generation
- Summarize what was generated
- Explain the SDK structure to the user
- Provide usage examples
- Suggest next steps (publishing, testing, CI/CD setup)

## Customization Guidance

When users need to customize the generated SDK, guide them on:

### Package Naming
```yaml
# In .speakeasy/gen.yaml
generation:
  packageName: my-api-sdk
  packageVersion: 1.0.0
```

### Method Naming
Use overlays with `x-speakeasy-name-override`:
```yaml
overlay: 1.0.0
actions:
  - target: "$.paths['/users'].get"
    update:
      x-speakeasy-name-override: listUsers
```

### Error Handling
```yaml
generation:
  errors:
    includeStatusCodes: true
    statusCodeMapping:
      4XX: ClientError
      5XX: ServerError
```

## Error Handling

Common issues and solutions:

| Error | Cause | Solution |
|-------|-------|----------|
| "Invalid schema" | Malformed OpenAPI | Run lint, fix validation errors |
| "Missing operationId" | Unnamed operations | Use suggest to generate IDs |
| "Unsupported feature" | Spec uses unsupported OpenAPI features | Create overlay to work around |
| "Generation failed" | Various | Check stderr, often missing dependencies |

## Example Interaction

```
User: I need to generate a Python SDK from my API spec

Agent: I'll help you set up a Python SDK. Let me first check your workspace...

[Uses speakeasy_status to check for existing config]

I see you have an OpenAPI spec at ./api/openapi.yaml. Let me validate it first.

[Uses speakeasy_lint]

The spec looks good with just 2 minor warnings. Would you like me to:
1. Set up the project and generate the SDK
2. Apply suggestions to improve operation names first
3. Show me the warnings in detail

User: Set up and generate

Agent: I'll initialize a Speakeasy project for Python SDK generation.

[Uses speakeasy_quickstart with schema=./api/openapi.yaml, target=python]

✅ Created .speakeasy/workflow.yaml

Now let me generate your SDK...

[Uses speakeasy_run]

✅ Python SDK generated successfully at ./sdk/python/

The SDK includes:
- `my_api/` - Main package
- `my_api/models/` - Pydantic models for all schemas
- `my_api/operations/` - API operation methods
- `README.md` - Usage documentation
- `pyproject.toml` - Package configuration

To use the SDK:
\`\`\`python
from my_api import MyAPI

client = MyAPI(api_key="your-key")
users = client.users.list()
\`\`\`

To regenerate after spec changes, just run `speakeasy run` or use `/speakeasy-run`.

Would you like me to help with anything else?
```
