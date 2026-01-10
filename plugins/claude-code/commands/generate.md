# /speakeasy-generate

Generate a production-ready SDK from an OpenAPI specification.

## Usage

```
/speakeasy-generate [schema] [target] [output]
```

## Arguments

- `schema` - Path or URL to the OpenAPI specification (optional, will prompt if not provided)
- `target` - Target language: typescript, python, go, java, csharp, php, ruby, swift, kotlin, terraform
- `output` - Output directory for the generated SDK

## Instructions

When the user invokes this command:

1. **Identify the OpenAPI spec**: Look for an OpenAPI specification in the current directory or ask the user to provide one. Common locations include:
   - `openapi.yaml` or `openapi.json` in the root
   - `api/` or `spec/` directories
   - `.speakeasy/workflow.yaml` may reference the spec location

2. **Determine the target language**: If not specified, ask the user which language they want. Show the available options:
   - `typescript` - TypeScript SDK with full type safety
   - `python` - Python SDK with type hints
   - `go` - Go SDK with strong typing
   - `java` - Java SDK
   - `csharp` - C# SDK
   - `php` - PHP SDK
   - `ruby` - Ruby SDK
   - `swift` - Swift SDK for iOS/macOS
   - `kotlin` - Kotlin SDK for Android/JVM
   - `terraform` - Terraform provider

3. **Set the output directory**: Default to `./sdk/<target>` if not specified.

4. **Run the generation**: Use the `speakeasy_generate` MCP tool with the gathered parameters.

5. **Post-generation**:
   - Show the user where the SDK was generated
   - Offer to explain the SDK structure
   - Suggest next steps (installing dependencies, running tests, etc.)

## Example

```
User: /speakeasy-generate ./api/openapi.yaml typescript ./packages/sdk

Response: I'll generate a TypeScript SDK from your OpenAPI spec...
[Uses speakeasy_generate tool]
Successfully generated TypeScript SDK at ./packages/sdk

The SDK includes:
- Full TypeScript types for all models
- Type-safe API client methods
- Automatic retry and error handling
- Documentation for all operations
```
