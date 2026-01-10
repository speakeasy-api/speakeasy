# /speakeasy-run

Execute a Speakeasy workflow to generate SDKs according to your workflow.yaml configuration.

## Usage

```
/speakeasy-run [target]
```

## Arguments

- `target` - Specific target to run (optional, runs all targets if not specified)

## Instructions

When the user invokes this command:

1. **Check for workflow configuration**: Look for `.speakeasy/workflow.yaml` in the current directory.

2. **If no workflow exists**:
   - Inform the user that no workflow is configured
   - Suggest using `/speakeasy-quickstart` to set one up
   - Or offer to create a workflow.yaml based on their needs

3. **If workflow exists**:
   - Parse and display the configured sources and targets
   - Ask which target(s) to run if not specified
   - Use the `speakeasy_run` MCP tool to execute

4. **During execution**:
   - Show progress as the workflow runs
   - Report any errors with actionable guidance

5. **After completion**:
   - Summarize what was generated
   - Show paths to generated SDKs
   - Suggest next steps (testing, publishing, etc.)

## Example

```
User: /speakeasy-run typescript-sdk

Response: Running Speakeasy workflow for typescript-sdk target...

📋 Workflow Configuration:
- Source: my-api (./openapi.yaml)
- Target: typescript-sdk → ./sdks/typescript

[Uses speakeasy_run tool]

✅ Workflow completed successfully!

Generated:
- TypeScript SDK at ./sdks/typescript
- 45 API operations
- 23 model types

Next steps:
1. cd ./sdks/typescript && npm install
2. npm run build
3. npm test
```

## Workflow File Reference

The workflow.yaml supports:
- Multiple sources (OpenAPI specs)
- Multiple targets (different SDK languages)
- Overlays for spec modifications
- Registry integration for versioning
