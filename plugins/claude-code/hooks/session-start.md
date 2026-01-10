# Session Start Hook

Automatically detect and load Speakeasy workspace context when a session begins.

## Trigger

This hook runs at `SessionStart` - when Claude Code initializes a new session.

## Behavior

### 1. Detect Speakeasy Workspace

Check for the presence of Speakeasy configuration:

```
.speakeasy/
├── workflow.yaml    # Workflow configuration
├── gen.yaml         # Generation settings
└── workflow.lock    # Lock file
```

### 2. If Speakeasy Project Detected

Display a brief context message:

```
📦 Speakeasy workspace detected

Sources:
  - my-api: ./openapi.yaml

Targets:
  - typescript-sdk → ./sdks/typescript
  - python-sdk → ./sdks/python

Commands: /speakeasy-run, /speakeasy-lint, /speakeasy-suggest
```

### 3. Load Relevant Context

When Speakeasy files are found:
- Note the configured sources and targets
- Check for any overlays in use
- Identify the OpenAPI spec location

### 4. If No Speakeasy Project

Silently skip - don't announce anything. Only show context when relevant.

## Implementation

```typescript
// Pseudo-code for hook logic
async function onSessionStart(context: SessionContext) {
  const workflowPath = '.speakeasy/workflow.yaml';

  if (await fileExists(workflowPath)) {
    const workflow = await readYaml(workflowPath);

    // Extract sources
    const sources = Object.entries(workflow.sources || {})
      .map(([name, config]) => `  - ${name}: ${config.inputs?.[0]?.location}`)
      .join('\n');

    // Extract targets
    const targets = Object.entries(workflow.targets || {})
      .map(([name, config]) => `  - ${name} (${config.target}) → ${config.output}`)
      .join('\n');

    return {
      contextMessage: `
📦 Speakeasy workspace detected

Sources:
${sources}

Targets:
${targets}

Use /speakeasy-run to generate SDKs, /speakeasy-lint to validate your spec.
      `.trim()
    };
  }

  return null; // No Speakeasy project
}
```

## Notes

- Keep the output minimal - just enough to orient the user
- Don't run any generation or validation automatically
- The context helps Claude understand the project structure for better assistance
