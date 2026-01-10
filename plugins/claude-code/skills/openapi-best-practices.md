# OpenAPI Best Practices for SDK Generation

This skill provides guidance on OpenAPI best practices that result in high-quality generated SDKs.

## Auto-Activation

This skill should be activated when the user is:
- Editing OpenAPI/Swagger specification files (`.yaml`, `.json` with OpenAPI content)
- Discussing API design decisions
- Working with files in `.speakeasy/` directory
- Generating or configuring SDKs

## Core Principles

### 1. Every Operation Needs an operationId

```yaml
# ❌ Bad - SDK will generate ugly method names
paths:
  /users:
    get:
      summary: List users

# ✅ Good - Clean method name in SDK
paths:
  /users:
    get:
      operationId: listUsers
      summary: List users
```

**Why**: The operationId directly becomes the method name in generated SDKs. Without it, generators create names from the path and method, resulting in verbose names like `get_users_list`.

### 2. Use Consistent Naming Conventions

```yaml
# ✅ Recommended patterns
operationId: listUsers      # List/collection operations
operationId: getUser        # Single resource retrieval
operationId: createUser     # Create operations
operationId: updateUser     # Full update (PUT)
operationId: patchUser      # Partial update (PATCH)
operationId: deleteUser     # Delete operations
```

### 3. Group Related Operations

Use `x-speakeasy-group` to organize SDK methods:

```yaml
paths:
  /users:
    get:
      operationId: list
      x-speakeasy-group: users
  /users/{id}:
    get:
      operationId: get
      x-speakeasy-group: users
```

Results in: `sdk.users.list()`, `sdk.users.get(id)`

### 4. Define Reusable Schemas

```yaml
# ❌ Bad - Inline schemas
paths:
  /users:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string

# ✅ Good - Referenced schemas
paths:
  /users:
    post:
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/CreateUserRequest'

components:
  schemas:
    CreateUserRequest:
      type: object
      required:
        - name
      properties:
        name:
          type: string
```

### 5. Include Descriptions Everywhere

```yaml
components:
  schemas:
    User:
      description: Represents a user in the system
      type: object
      properties:
        id:
          type: string
          format: uuid
          description: Unique identifier for the user
        email:
          type: string
          format: email
          description: User's email address, must be unique
```

### 6. Define All Response Types

```yaml
paths:
  /users/{id}:
    get:
      responses:
        '200':
          description: User found
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/User'
        '404':
          description: User not found
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
        '500':
          description: Internal server error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
```

### 7. Use Proper Data Types

```yaml
# ✅ Use specific formats
properties:
  id:
    type: string
    format: uuid
  created_at:
    type: string
    format: date-time
  price:
    type: number
    format: double
  count:
    type: integer
    format: int32
  large_count:
    type: integer
    format: int64
```

## Speakeasy-Specific Extensions

### x-speakeasy-name-override
Override generated names:
```yaml
x-speakeasy-name-override: betterMethodName
```

### x-speakeasy-retries
Configure automatic retries:
```yaml
x-speakeasy-retries:
  strategy: backoff
  backoff:
    initialInterval: 500
    maxInterval: 60000
    maxElapsedTime: 3600000
    exponent: 1.5
  statusCodes:
    - 5XX
  retryConnectionErrors: true
```

### x-speakeasy-pagination
Enable automatic pagination:
```yaml
x-speakeasy-pagination:
  type: offsetLimit
  inputs:
    - name: offset
      in: query
    - name: limit
      in: query
  outputs:
    results: $.data
```

### x-speakeasy-errors
Define typed errors:
```yaml
x-speakeasy-errors:
  statusCodes:
    - 4XX
    - 5XX
```

## Anti-Patterns to Avoid

### 1. Path Parameters in Query
```yaml
# ❌ Bad
/users?id={id}

# ✅ Good
/users/{id}
```

### 2. Verbs in Paths
```yaml
# ❌ Bad
/users/create
/users/delete/{id}

# ✅ Good - Use HTTP methods
POST /users
DELETE /users/{id}
```

### 3. Inconsistent Pluralization
```yaml
# ❌ Bad - Mixed singular/plural
/user/{id}
/orders

# ✅ Good - Consistent plural
/users/{id}
/orders
```

### 4. Version in Every Path
```yaml
# ❌ Verbose
/api/v1/users
/api/v1/orders

# ✅ Better - Use server URL
servers:
  - url: https://api.example.com/v1
paths:
  /users:
  /orders:
```

## Quick Reference Card

| Element | Best Practice |
|---------|--------------|
| operationId | camelCase verb+noun: `listUsers`, `createOrder` |
| Schemas | PascalCase: `User`, `OrderRequest` |
| Properties | snake_case or camelCase (be consistent) |
| Paths | lowercase, plural nouns: `/users`, `/orders` |
| Parameters | camelCase: `userId`, `pageSize` |

## When Reviewing OpenAPI Specs

Always check for:
1. ✓ All operations have unique operationIds
2. ✓ Schemas are defined in components (not inline)
3. ✓ All properties have descriptions
4. ✓ Error responses are defined
5. ✓ Proper data types and formats used
6. ✓ Consistent naming conventions
7. ✓ No deprecated patterns (Swagger 2.0, etc.)
