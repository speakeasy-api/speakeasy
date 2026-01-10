# /speakeasy-lint

Validate and lint an OpenAPI specification for errors, warnings, and best practice violations.

## Usage

```
/speakeasy-lint [schema]
```

## Arguments

- `schema` - Path to the OpenAPI specification file (optional, will auto-detect)

## Instructions

When the user invokes this command:

1. **Find the OpenAPI spec**: If not provided, search for OpenAPI specs in common locations:
   - `openapi.yaml`, `openapi.json`
   - `swagger.yaml`, `swagger.json`
   - `api.yaml`, `api.json`
   - Check `.speakeasy/workflow.yaml` for configured sources

2. **Run the linter**: Use the `speakeasy_lint` MCP tool.

3. **Present the results** in a clear, actionable format:
   - **Errors** (must fix): Issues that will prevent SDK generation
   - **Warnings** (should fix): Issues that may cause problems
   - **Hints** (nice to fix): Suggestions for improvement

4. **Offer to help fix issues**: For each error or warning, offer to:
   - Explain what the issue means
   - Show how to fix it
   - Create an overlay to fix it without modifying the original spec

## Example Output Format

```
## Lint Results for openapi.yaml

### ❌ Errors (2)
1. **Missing operationId** at `GET /users`
   - Every operation should have a unique operationId
   - Suggested fix: Add `operationId: getUsers`

2. **Invalid schema reference** at `POST /orders` response
   - Reference `#/components/schemas/OrderResponse` not found
   - Check for typos in the schema name

### ⚠️ Warnings (3)
1. **Missing description** for `User` schema
2. **No error responses defined** for `DELETE /users/{id}`
3. **Deprecated: using swagger 2.0** - Consider upgrading to OpenAPI 3.x

### 💡 Hints (1)
1. Consider adding `x-speakeasy-name-override` for cleaner method names

Would you like me to help fix any of these issues?
```
