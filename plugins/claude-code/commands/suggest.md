# /speakeasy-suggest

Get AI-powered suggestions to improve your OpenAPI specification.

## Usage

```
/speakeasy-suggest [schema] [type]
```

## Arguments

- `schema` - Path to the OpenAPI specification (optional, will auto-detect)
- `type` - Type of suggestions: `operation-ids`, `error-types`, or `all` (default: all)

## Instructions

When the user invokes this command:

1. **Locate the OpenAPI spec**: Find the spec file or ask the user.

2. **Run suggestions**: Use the `speakeasy_suggest` MCP tool with the appropriate type:
   - `operation-ids` - Suggests better, more consistent operation IDs
   - `error-types` - Suggests structured error response schemas
   - `all` - Runs all suggestion types

3. **Present suggestions clearly**:
   - Show the current value vs. suggested improvement
   - Explain why the suggestion improves the API
   - Offer to apply suggestions via an overlay

4. **Apply suggestions**: If the user wants to apply suggestions:
   - Create an overlay file with the changes
   - Show how to integrate the overlay into their workflow
   - Do NOT modify the original spec directly

## Example

```
User: /speakeasy-suggest ./openapi.yaml operation-ids

Response: Analyzing your OpenAPI spec for operation ID improvements...

## Suggested Operation ID Improvements

| Endpoint | Current | Suggested | Reason |
|----------|---------|-----------|--------|
| GET /users | get_users_list | listUsers | More idiomatic, follows SDK naming conventions |
| POST /users | create_new_user | createUser | Removes redundant "new" |
| GET /users/{id} | getUserById | getUser | The {id} parameter implies "by ID" |
| DELETE /api/v1/users/{id} | api_v1_delete_user | deleteUser | Removes version prefix noise |

Would you like me to create an overlay file to apply these suggestions?
```

## Notes

- Suggestions are generated using AI and should be reviewed before applying
- The overlay approach preserves your original spec while applying improvements
- Suggestions follow Speakeasy's best practices for SDK generation
