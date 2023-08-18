# AuthNew

## Overview

Endpoints for testing authentication.

### Available Operations

* [APIKeyAuthGlobalNew](#apikeyauthglobalnew)
* [BasicAuthNew](#basicauthnew)
* [MultipleMixedOptionsAuth](#multiplemixedoptionsauth)
* [MultipleMixedSchemeAuth](#multiplemixedschemeauth)
* [MultipleOptionsWithMixedSchemesAuth](#multipleoptionswithmixedschemesauth)
* [MultipleOptionsWithSimpleSchemesAuth](#multipleoptionswithsimpleschemesauth)
* [MultipleSimpleOptionsAuth](#multiplesimpleoptionsauth)
* [MultipleSimpleSchemeAuth](#multiplesimpleschemeauth)
* [Oauth2AuthNew](#oauth2authnew)
* [OpenIDConnectAuthNew](#openidconnectauthnew)

## APIKeyAuthGlobalNew

### Example Usage

```go
package main

import(
	"context"
	"log"
	"openapi"
	"openapi/pkg/models/shared"
)

func main() {
    s := sdk.New(
        sdk.WithSecurity(shared.Security{
            APIKeyAuth: sdk.String("Token YOUR_API_KEY"),
        }),
        sdk.WithGlobalPathParam(100),
        sdk.WithGlobalQueryParam("some example global query param"),
    )

    ctx := context.Background()
    res, err := s.AuthNew.APIKeyAuthGlobalNew(ctx, shared.AuthServiceRequestBody{
        BasicAuth: &shared.AuthServiceRequestBodyBasicAuth{
            Password: "ipsam",
            Username: "Makayla9",
        },
        HeaderAuth: []shared.AuthServiceRequestBodyHeaderAuth{
            shared.AuthServiceRequestBodyHeaderAuth{
                ExpectedValue: "temporibus",
                HeaderName: "laborum",
            },
            shared.AuthServiceRequestBodyHeaderAuth{
                ExpectedValue: "quasi",
                HeaderName: "reiciendis",
            },
            shared.AuthServiceRequestBodyHeaderAuth{
                ExpectedValue: "voluptatibus",
                HeaderName: "vero",
            },
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.StatusCode == http.StatusOK {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                      | Type                                                                           | Required                                                                       | Description                                                                    |
| ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------ |
| `ctx`                                                                          | [context.Context](https://pkg.go.dev/context#Context)                          | :heavy_check_mark:                                                             | The context to use for the request.                                            |
| `request`                                                                      | [shared.AuthServiceRequestBody](../../models/shared/authservicerequestbody.md) | :heavy_check_mark:                                                             | The request object to use for the request.                                     |
| `opts`                                                                         | [][operations.Option](../../models/operations/option.md)                       | :heavy_minus_sign:                                                             | The options for this request.                                                  |


### Response

**[*operations.APIKeyAuthGlobalNewResponse](../../models/operations/apikeyauthglobalnewresponse.md), error**


## BasicAuthNew

### Example Usage

```go
package main

import(
	"context"
	"log"
	"openapi"
	"openapi/pkg/models/shared"
	"openapi/pkg/models/operations"
)

func main() {
    s := sdk.New(
        sdk.WithGlobalPathParam(100),
        sdk.WithGlobalQueryParam("some example global query param"),
    )

    ctx := context.Background()
    res, err := s.AuthNew.BasicAuthNew(ctx, shared.AuthServiceRequestBody{
        BasicAuth: &shared.AuthServiceRequestBodyBasicAuth{
            Password: "nihil",
            Username: "John60",
        },
        HeaderAuth: []shared.AuthServiceRequestBodyHeaderAuth{
            shared.AuthServiceRequestBodyHeaderAuth{
                ExpectedValue: "cum",
                HeaderName: "perferendis",
            },
            shared.AuthServiceRequestBodyHeaderAuth{
                ExpectedValue: "doloremque",
                HeaderName: "reprehenderit",
            },
        },
    }, operations.BasicAuthNewSecurity{
        Password: "YOUR_PASSWORD",
        Username: "YOUR_USERNAME",
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.StatusCode == http.StatusOK {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                          | Type                                                                               | Required                                                                           | Description                                                                        |
| ---------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| `ctx`                                                                              | [context.Context](https://pkg.go.dev/context#Context)                              | :heavy_check_mark:                                                                 | The context to use for the request.                                                |
| `request`                                                                          | [shared.AuthServiceRequestBody](../../models/shared/authservicerequestbody.md)     | :heavy_check_mark:                                                                 | The request object to use for the request.                                         |
| `security`                                                                         | [operations.BasicAuthNewSecurity](../../models/operations/basicauthnewsecurity.md) | :heavy_check_mark:                                                                 | The security requirements to use for the request.                                  |
| `opts`                                                                             | [][operations.Option](../../models/operations/option.md)                           | :heavy_minus_sign:                                                                 | The options for this request.                                                      |


### Response

**[*operations.BasicAuthNewResponse](../../models/operations/basicauthnewresponse.md), error**


## MultipleMixedOptionsAuth

### Example Usage

```go
package main

import(
	"context"
	"log"
	"openapi"
	"openapi/pkg/models/shared"
	"openapi/pkg/models/operations"
)

func main() {
    s := sdk.New(
        sdk.WithGlobalPathParam(100),
        sdk.WithGlobalQueryParam("some example global query param"),
    )

    ctx := context.Background()
    res, err := s.AuthNew.MultipleMixedOptionsAuth(ctx, shared.AuthServiceRequestBody{
        BasicAuth: &shared.AuthServiceRequestBodyBasicAuth{
            Password: "ut",
            Username: "Wilfrid.Carter",
        },
        HeaderAuth: []shared.AuthServiceRequestBodyHeaderAuth{
            shared.AuthServiceRequestBodyHeaderAuth{
                ExpectedValue: "dicta",
                HeaderName: "harum",
            },
            shared.AuthServiceRequestBodyHeaderAuth{
                ExpectedValue: "enim",
                HeaderName: "accusamus",
            },
        },
    }, operations.MultipleMixedOptionsAuthSecurity{
        APIKeyAuthNew: sdk.String("Token <YOUR_API_KEY>"),
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.StatusCode == http.StatusOK {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                                                  | Type                                                                                                       | Required                                                                                                   | Description                                                                                                |
| ---------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| `ctx`                                                                                                      | [context.Context](https://pkg.go.dev/context#Context)                                                      | :heavy_check_mark:                                                                                         | The context to use for the request.                                                                        |
| `request`                                                                                                  | [shared.AuthServiceRequestBody](../../models/shared/authservicerequestbody.md)                             | :heavy_check_mark:                                                                                         | The request object to use for the request.                                                                 |
| `security`                                                                                                 | [operations.MultipleMixedOptionsAuthSecurity](../../models/operations/multiplemixedoptionsauthsecurity.md) | :heavy_check_mark:                                                                                         | The security requirements to use for the request.                                                          |
| `opts`                                                                                                     | [][operations.Option](../../models/operations/option.md)                                                   | :heavy_minus_sign:                                                                                         | The options for this request.                                                                              |


### Response

**[*operations.MultipleMixedOptionsAuthResponse](../../models/operations/multiplemixedoptionsauthresponse.md), error**


## MultipleMixedSchemeAuth

### Example Usage

```go
package main

import(
	"context"
	"log"
	"openapi"
	"openapi/pkg/models/shared"
	"openapi/pkg/models/operations"
)

func main() {
    s := sdk.New(
        sdk.WithGlobalPathParam(100),
        sdk.WithGlobalQueryParam("some example global query param"),
    )

    ctx := context.Background()
    res, err := s.AuthNew.MultipleMixedSchemeAuth(ctx, shared.AuthServiceRequestBody{
        BasicAuth: &shared.AuthServiceRequestBodyBasicAuth{
            Password: "commodi",
            Username: "Terrill69",
        },
        HeaderAuth: []shared.AuthServiceRequestBodyHeaderAuth{
            shared.AuthServiceRequestBodyHeaderAuth{
                ExpectedValue: "excepturi",
                HeaderName: "pariatur",
            },
            shared.AuthServiceRequestBodyHeaderAuth{
                ExpectedValue: "modi",
                HeaderName: "praesentium",
            },
            shared.AuthServiceRequestBodyHeaderAuth{
                ExpectedValue: "rem",
                HeaderName: "voluptates",
            },
        },
    }, operations.MultipleMixedSchemeAuthSecurity{
        APIKeyAuthNew: "Token <YOUR_API_KEY>",
        BasicAuth: shared.SchemeBasicAuth{
            Password: "YOUR_PASSWORD",
            Username: "YOUR_USERNAME",
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.StatusCode == http.StatusOK {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                                                | Type                                                                                                     | Required                                                                                                 | Description                                                                                              |
| -------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- |
| `ctx`                                                                                                    | [context.Context](https://pkg.go.dev/context#Context)                                                    | :heavy_check_mark:                                                                                       | The context to use for the request.                                                                      |
| `request`                                                                                                | [shared.AuthServiceRequestBody](../../models/shared/authservicerequestbody.md)                           | :heavy_check_mark:                                                                                       | The request object to use for the request.                                                               |
| `security`                                                                                               | [operations.MultipleMixedSchemeAuthSecurity](../../models/operations/multiplemixedschemeauthsecurity.md) | :heavy_check_mark:                                                                                       | The security requirements to use for the request.                                                        |
| `opts`                                                                                                   | [][operations.Option](../../models/operations/option.md)                                                 | :heavy_minus_sign:                                                                                       | The options for this request.                                                                            |


### Response

**[*operations.MultipleMixedSchemeAuthResponse](../../models/operations/multiplemixedschemeauthresponse.md), error**


## MultipleOptionsWithMixedSchemesAuth

### Example Usage

```go
package main

import(
	"context"
	"log"
	"openapi"
	"openapi/pkg/models/shared"
	"openapi/pkg/models/operations"
)

func main() {
    s := sdk.New(
        sdk.WithGlobalPathParam(100),
        sdk.WithGlobalQueryParam("some example global query param"),
    )

    ctx := context.Background()
    res, err := s.AuthNew.MultipleOptionsWithMixedSchemesAuth(ctx, shared.AuthServiceRequestBody{
        BasicAuth: &shared.AuthServiceRequestBodyBasicAuth{
            Password: "quasi",
            Username: "Thelma92",
        },
        HeaderAuth: []shared.AuthServiceRequestBodyHeaderAuth{
            shared.AuthServiceRequestBodyHeaderAuth{
                ExpectedValue: "enim",
                HeaderName: "consequatur",
            },
            shared.AuthServiceRequestBodyHeaderAuth{
                ExpectedValue: "est",
                HeaderName: "quibusdam",
            },
        },
    }, operations.MultipleOptionsWithMixedSchemesAuthSecurity{
        Option1: &operations.MultipleOptionsWithMixedSchemesAuthSecurityOption1{
            APIKeyAuthNew: "Token <YOUR_API_KEY>",
            Oauth2: "Bearer YOUR_OAUTH2_TOKEN",
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.StatusCode == http.StatusOK {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                                                                        | Type                                                                                                                             | Required                                                                                                                         | Description                                                                                                                      |
| -------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| `ctx`                                                                                                                            | [context.Context](https://pkg.go.dev/context#Context)                                                                            | :heavy_check_mark:                                                                                                               | The context to use for the request.                                                                                              |
| `request`                                                                                                                        | [shared.AuthServiceRequestBody](../../models/shared/authservicerequestbody.md)                                                   | :heavy_check_mark:                                                                                                               | The request object to use for the request.                                                                                       |
| `security`                                                                                                                       | [operations.MultipleOptionsWithMixedSchemesAuthSecurity](../../models/operations/multipleoptionswithmixedschemesauthsecurity.md) | :heavy_check_mark:                                                                                                               | The security requirements to use for the request.                                                                                |
| `opts`                                                                                                                           | [][operations.Option](../../models/operations/option.md)                                                                         | :heavy_minus_sign:                                                                                                               | The options for this request.                                                                                                    |


### Response

**[*operations.MultipleOptionsWithMixedSchemesAuthResponse](../../models/operations/multipleoptionswithmixedschemesauthresponse.md), error**


## MultipleOptionsWithSimpleSchemesAuth

### Example Usage

```go
package main

import(
	"context"
	"log"
	"openapi"
	"openapi/pkg/models/shared"
	"openapi/pkg/models/operations"
)

func main() {
    s := sdk.New(
        sdk.WithGlobalPathParam(100),
        sdk.WithGlobalQueryParam("some example global query param"),
    )

    ctx := context.Background()
    res, err := s.AuthNew.MultipleOptionsWithSimpleSchemesAuth(ctx, shared.AuthServiceRequestBody{
        BasicAuth: &shared.AuthServiceRequestBodyBasicAuth{
            Password: "explicabo",
            Username: "Luther.Rau26",
        },
        HeaderAuth: []shared.AuthServiceRequestBodyHeaderAuth{
            shared.AuthServiceRequestBodyHeaderAuth{
                ExpectedValue: "aliquid",
                HeaderName: "cupiditate",
            },
        },
    }, operations.MultipleOptionsWithSimpleSchemesAuthSecurity{
        Option1: &operations.MultipleOptionsWithSimpleSchemesAuthSecurityOption1{
            APIKeyAuthNew: "Token <YOUR_API_KEY>",
            Oauth2: "Bearer YOUR_OAUTH2_TOKEN",
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.StatusCode == http.StatusOK {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                                                                          | Type                                                                                                                               | Required                                                                                                                           | Description                                                                                                                        |
| ---------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| `ctx`                                                                                                                              | [context.Context](https://pkg.go.dev/context#Context)                                                                              | :heavy_check_mark:                                                                                                                 | The context to use for the request.                                                                                                |
| `request`                                                                                                                          | [shared.AuthServiceRequestBody](../../models/shared/authservicerequestbody.md)                                                     | :heavy_check_mark:                                                                                                                 | The request object to use for the request.                                                                                         |
| `security`                                                                                                                         | [operations.MultipleOptionsWithSimpleSchemesAuthSecurity](../../models/operations/multipleoptionswithsimpleschemesauthsecurity.md) | :heavy_check_mark:                                                                                                                 | The security requirements to use for the request.                                                                                  |
| `opts`                                                                                                                             | [][operations.Option](../../models/operations/option.md)                                                                           | :heavy_minus_sign:                                                                                                                 | The options for this request.                                                                                                      |


### Response

**[*operations.MultipleOptionsWithSimpleSchemesAuthResponse](../../models/operations/multipleoptionswithsimpleschemesauthresponse.md), error**


## MultipleSimpleOptionsAuth

### Example Usage

```go
package main

import(
	"context"
	"log"
	"openapi"
	"openapi/pkg/models/shared"
	"openapi/pkg/models/operations"
)

func main() {
    s := sdk.New(
        sdk.WithGlobalPathParam(100),
        sdk.WithGlobalQueryParam("some example global query param"),
    )

    ctx := context.Background()
    res, err := s.AuthNew.MultipleSimpleOptionsAuth(ctx, shared.AuthServiceRequestBody{
        BasicAuth: &shared.AuthServiceRequestBodyBasicAuth{
            Password: "quos",
            Username: "Aiyana.Cummerata0",
        },
        HeaderAuth: []shared.AuthServiceRequestBodyHeaderAuth{
            shared.AuthServiceRequestBodyHeaderAuth{
                ExpectedValue: "dolorum",
                HeaderName: "excepturi",
            },
        },
    }, operations.MultipleSimpleOptionsAuthSecurity{
        APIKeyAuthNew: sdk.String("Token <YOUR_API_KEY>"),
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.StatusCode == http.StatusOK {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                                                    | Type                                                                                                         | Required                                                                                                     | Description                                                                                                  |
| ------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------ |
| `ctx`                                                                                                        | [context.Context](https://pkg.go.dev/context#Context)                                                        | :heavy_check_mark:                                                                                           | The context to use for the request.                                                                          |
| `request`                                                                                                    | [shared.AuthServiceRequestBody](../../models/shared/authservicerequestbody.md)                               | :heavy_check_mark:                                                                                           | The request object to use for the request.                                                                   |
| `security`                                                                                                   | [operations.MultipleSimpleOptionsAuthSecurity](../../models/operations/multiplesimpleoptionsauthsecurity.md) | :heavy_check_mark:                                                                                           | The security requirements to use for the request.                                                            |
| `opts`                                                                                                       | [][operations.Option](../../models/operations/option.md)                                                     | :heavy_minus_sign:                                                                                           | The options for this request.                                                                                |


### Response

**[*operations.MultipleSimpleOptionsAuthResponse](../../models/operations/multiplesimpleoptionsauthresponse.md), error**


## MultipleSimpleSchemeAuth

### Example Usage

```go
package main

import(
	"context"
	"log"
	"openapi"
	"openapi/pkg/models/shared"
	"openapi/pkg/models/operations"
)

func main() {
    s := sdk.New(
        sdk.WithGlobalPathParam(100),
        sdk.WithGlobalQueryParam("some example global query param"),
    )

    ctx := context.Background()
    res, err := s.AuthNew.MultipleSimpleSchemeAuth(ctx, shared.AuthServiceRequestBody{
        BasicAuth: &shared.AuthServiceRequestBodyBasicAuth{
            Password: "tempora",
            Username: "Mckayla96",
        },
        HeaderAuth: []shared.AuthServiceRequestBodyHeaderAuth{
            shared.AuthServiceRequestBodyHeaderAuth{
                ExpectedValue: "non",
                HeaderName: "eligendi",
            },
            shared.AuthServiceRequestBodyHeaderAuth{
                ExpectedValue: "sint",
                HeaderName: "aliquid",
            },
        },
    }, operations.MultipleSimpleSchemeAuthSecurity{
        APIKeyAuthNew: "Token <YOUR_API_KEY>",
        Oauth2: "Bearer YOUR_OAUTH2_TOKEN",
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.StatusCode == http.StatusOK {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                                                  | Type                                                                                                       | Required                                                                                                   | Description                                                                                                |
| ---------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| `ctx`                                                                                                      | [context.Context](https://pkg.go.dev/context#Context)                                                      | :heavy_check_mark:                                                                                         | The context to use for the request.                                                                        |
| `request`                                                                                                  | [shared.AuthServiceRequestBody](../../models/shared/authservicerequestbody.md)                             | :heavy_check_mark:                                                                                         | The request object to use for the request.                                                                 |
| `security`                                                                                                 | [operations.MultipleSimpleSchemeAuthSecurity](../../models/operations/multiplesimpleschemeauthsecurity.md) | :heavy_check_mark:                                                                                         | The security requirements to use for the request.                                                          |
| `opts`                                                                                                     | [][operations.Option](../../models/operations/option.md)                                                   | :heavy_minus_sign:                                                                                         | The options for this request.                                                                              |


### Response

**[*operations.MultipleSimpleSchemeAuthResponse](../../models/operations/multiplesimpleschemeauthresponse.md), error**


## Oauth2AuthNew

### Example Usage

```go
package main

import(
	"context"
	"log"
	"openapi"
	"openapi/pkg/models/shared"
	"openapi/pkg/models/operations"
)

func main() {
    s := sdk.New(
        sdk.WithGlobalPathParam(100),
        sdk.WithGlobalQueryParam("some example global query param"),
    )

    ctx := context.Background()
    res, err := s.AuthNew.Oauth2AuthNew(ctx, shared.AuthServiceRequestBody{
        BasicAuth: &shared.AuthServiceRequestBodyBasicAuth{
            Password: "provident",
            Username: "Sonya.Marquardt",
        },
        HeaderAuth: []shared.AuthServiceRequestBodyHeaderAuth{
            shared.AuthServiceRequestBodyHeaderAuth{
                ExpectedValue: "a",
                HeaderName: "dolorum",
            },
            shared.AuthServiceRequestBodyHeaderAuth{
                ExpectedValue: "in",
                HeaderName: "in",
            },
            shared.AuthServiceRequestBodyHeaderAuth{
                ExpectedValue: "illum",
                HeaderName: "maiores",
            },
            shared.AuthServiceRequestBodyHeaderAuth{
                ExpectedValue: "rerum",
                HeaderName: "dicta",
            },
        },
    }, operations.Oauth2AuthNewSecurity{
        Oauth2: "Bearer YOUR_OAUTH2_TOKEN",
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.StatusCode == http.StatusOK {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                            | Type                                                                                 | Required                                                                             | Description                                                                          |
| ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------ |
| `ctx`                                                                                | [context.Context](https://pkg.go.dev/context#Context)                                | :heavy_check_mark:                                                                   | The context to use for the request.                                                  |
| `request`                                                                            | [shared.AuthServiceRequestBody](../../models/shared/authservicerequestbody.md)       | :heavy_check_mark:                                                                   | The request object to use for the request.                                           |
| `security`                                                                           | [operations.Oauth2AuthNewSecurity](../../models/operations/oauth2authnewsecurity.md) | :heavy_check_mark:                                                                   | The security requirements to use for the request.                                    |
| `opts`                                                                               | [][operations.Option](../../models/operations/option.md)                             | :heavy_minus_sign:                                                                   | The options for this request.                                                        |


### Response

**[*operations.Oauth2AuthNewResponse](../../models/operations/oauth2authnewresponse.md), error**


## OpenIDConnectAuthNew

### Example Usage

```go
package main

import(
	"context"
	"log"
	"openapi"
	"openapi/pkg/models/shared"
	"openapi/pkg/models/operations"
)

func main() {
    s := sdk.New(
        sdk.WithGlobalPathParam(100),
        sdk.WithGlobalQueryParam("some example global query param"),
    )

    ctx := context.Background()
    res, err := s.AuthNew.OpenIDConnectAuthNew(ctx, shared.AuthServiceRequestBody{
        BasicAuth: &shared.AuthServiceRequestBodyBasicAuth{
            Password: "magnam",
            Username: "Obie.Schulist",
        },
        HeaderAuth: []shared.AuthServiceRequestBodyHeaderAuth{
            shared.AuthServiceRequestBodyHeaderAuth{
                ExpectedValue: "accusamus",
                HeaderName: "non",
            },
            shared.AuthServiceRequestBodyHeaderAuth{
                ExpectedValue: "occaecati",
                HeaderName: "enim",
            },
            shared.AuthServiceRequestBodyHeaderAuth{
                ExpectedValue: "accusamus",
                HeaderName: "delectus",
            },
        },
    }, operations.OpenIDConnectAuthNewSecurity{
        OpenIDConnect: "Bearer YOUR_OPENID_TOKEN",
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.StatusCode == http.StatusOK {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                                          | Type                                                                                               | Required                                                                                           | Description                                                                                        |
| -------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------- |
| `ctx`                                                                                              | [context.Context](https://pkg.go.dev/context#Context)                                              | :heavy_check_mark:                                                                                 | The context to use for the request.                                                                |
| `request`                                                                                          | [shared.AuthServiceRequestBody](../../models/shared/authservicerequestbody.md)                     | :heavy_check_mark:                                                                                 | The request object to use for the request.                                                         |
| `security`                                                                                         | [operations.OpenIDConnectAuthNewSecurity](../../models/operations/openidconnectauthnewsecurity.md) | :heavy_check_mark:                                                                                 | The security requirements to use for the request.                                                  |
| `opts`                                                                                             | [][operations.Option](../../models/operations/option.md)                                           | :heavy_minus_sign:                                                                                 | The options for this request.                                                                      |


### Response

**[*operations.OpenIDConnectAuthNewResponse](../../models/operations/openidconnectauthnewresponse.md), error**

