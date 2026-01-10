# /speakeasy-quickstart

Initialize a new Speakeasy project with workflow configuration.

## Usage

```
/speakeasy-quickstart [schema] [target]
```

## Arguments

- `schema` - Path or URL to your OpenAPI specification
- `target` - Target language for SDK generation

## Instructions

When the user invokes this command:

1. **Gather information**:
   - If `schema` not provided, search for OpenAPI specs or ask for the path/URL
   - If `target` not provided, ask which language they want to generate

2. **Create the project structure**:
   - Use `speakeasy_quickstart` MCP tool to create `.speakeasy/workflow.yaml`
   - Explain the created configuration

3. **Validate the setup**:
   - Run a quick lint on the OpenAPI spec
   - Report any issues that might affect generation

4. **Guide next steps**:
   - Explain how to run the workflow
   - Suggest customizations they might want
   - Point to documentation for advanced features

## Example

```
User: /speakeasy-quickstart https://api.example.com/openapi.yaml python

Response: Setting up Speakeasy project for Python SDK generation...

✅ Created .speakeasy/workflow.yaml

Configuration:
```yaml
workflowVersion: 1.0.0
speakeasyVersion: latest
sources:
  my-api:
    inputs:
      - location: https://api.example.com/openapi.yaml
targets:
  python-sdk:
    target: python
    source: my-api
    output: ./sdk/python
```

Your project is ready! Next steps:

1. **Generate your SDK**: Run `/speakeasy-run` or `speakeasy run`
2. **Customize generation**: Edit `.speakeasy/gen.yaml` for package name, etc.
3. **Add more targets**: Add TypeScript, Go, or other SDKs to workflow.yaml

Would you like me to run the first generation now?
```

## Multiple Targets

To set up multiple SDK targets at once:

```
/speakeasy-quickstart ./api.yaml typescript,python,go
```

This creates a workflow with all three targets configured.
