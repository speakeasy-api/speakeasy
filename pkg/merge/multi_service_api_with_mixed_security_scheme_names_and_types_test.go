package merge

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_merge_WithNamespaces_multi_service_API_with_mixed_security_scheme_names_and_types(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		// storage: simple JWT bearer, no global security
		[]byte(`openapi: 3.1
paths:
  /secrets:
    get:
      operationId: listSecrets
      security:
        - BearerAuth: []
      responses:
        200:
          description: OK
components:
  securitySchemes:
    BearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT`),
		// admin: simple JWT bearer, no global security
		[]byte(`openapi: 3.1
paths:
  /tenants:
    get:
      operationId: listTenants
      security:
        - BearerAuth: []
      responses:
        200:
          description: OK
components:
  securitySchemes:
    BearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT`),
		// users: oauth2 CC with SCIM scopes + basicAuth, global security
		[]byte(`openapi: 3.1
security:
  - bearerAuth: []
paths:
  /Users:
    get:
      operationId: listUsers
      responses:
        200:
          description: OK
components:
  securitySchemes:
    bearerAuth:
      type: oauth2
      description: OAuth 2.0 Bearer token for user management
      flows:
        clientCredentials:
          tokenUrl: https://idp.example.com/oauth2/token
          scopes:
            Users:Read: Read user profiles
            Users:Write: Create and update users
            Users:Groups: Manage group membership
    basicAuth:
      type: http
      scheme: basic`),
		// tokens: oauth2 CC with token management scopes + basicAuth, global security
		[]byte(`openapi: 3.1
security:
  - bearerAuth: []
paths:
  /oauth2/clients:
    get:
      operationId: listClients
      responses:
        200:
          description: OK
components:
  securitySchemes:
    bearerAuth:
      type: oauth2
      description: OAuth 2.0 Bearer token for token management
      flows:
        clientCredentials:
          tokenUrl: https://idp.example.com/oauth2/token
          scopes:
            Tokens:Manage: Manage OAuth client registrations
            Tokens:Read: Read token and client information
    basicAuth:
      type: http
      scheme: basic`),
		// flows: oauth2 CC with empty scopes (role-based), global security: []
		[]byte(`openapi: 3.1
security: []
paths:
  /login/flow:
    post:
      operationId: startLoginFlow
      security:
        - bearerAuth: []
      responses:
        200:
          description: OK
components:
  securitySchemes:
    bearerAuth:
      type: oauth2
      description: Bearer token for auth flow operations
      flows:
        clientCredentials:
          tokenUrl: https://idp.example.com/oauth2/token
          scopes: {}`),
		// assistant: simple JWT bearer (lowercase name), no global security
		[]byte(`openapi: 3.1
paths:
  /assistant/chat:
    post:
      operationId: chat
      security:
        - bearerAuth: []
      responses:
        200:
          description: OK
components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT`),
		// registry: HTTPBearer (different name, no bearerFormat), no global security
		[]byte(`openapi: 3.1
paths:
  /skills:
    get:
      operationId: listSkills
      security:
        - HTTPBearer: []
      responses:
        200:
          description: OK
components:
  securitySchemes:
    HTTPBearer:
      type: http
      scheme: bearer`),
	}
	namespaces := []string{"storage", "admin", "users", "tokens", "flows", "assistant", "registry"}

	got, err := merge(t.Context(), inSchemas, namespaces, true)
	require.NoError(t, err)
	assert.Equal(t, `openapi: "3.1"
paths:
  /secrets:
    get:
      operationId: listSecrets
      security:
        - BearerAuth: []
      responses:
        200:
          description: OK
  /tenants:
    get:
      operationId: listTenants
      security:
        - BearerAuth: []
      responses:
        "200":
          description: OK
  /Users:
    get:
      operationId: listUsers
      security:
        - bearerAuth: []
      responses:
        "200":
          description: OK
  /oauth2/clients:
    get:
      operationId: listClients
      security:
        - bearerAuth: []
      responses:
        "200":
          description: OK
  /login/flow:
    post:
      operationId: startLoginFlow
      security:
        - bearerAuth: []
      responses:
        "200":
          description: OK
  /assistant/chat:
    post:
      operationId: chat
      security:
        - BearerAuth: []
      responses:
        "200":
          description: OK
  /skills:
    get:
      operationId: listSkills
      security:
        - HTTPBearer: []
      responses:
        "200":
          description: OK
components:
  securitySchemes:
    BearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT
    basicAuth:
      type: http
      scheme: basic
    bearerAuth:
      type: oauth2
      description: |-
        OAuth 2.0 Bearer token for user management
        OAuth 2.0 Bearer token for token management
        Bearer token for auth flow operations
      flows:
        clientCredentials:
          tokenUrl: https://idp.example.com/oauth2/token
          scopes:
            Users:Read: Read user profiles
            Users:Write: Create and update users
            Users:Groups: Manage group membership
            Tokens:Manage: Manage OAuth client registrations
            Tokens:Read: Read token and client information
    HTTPBearer:
      type: http
      scheme: bearer
info:
  title: ""
  version: ""
`, string(got))
}
