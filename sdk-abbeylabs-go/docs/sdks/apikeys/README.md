# APIKeys

## Overview

API Keys are used to authenticate to the Abbey API.


<https://docs.abbey.io/product/managing-api-keys>
### Available Operations

* [CreateAPIKey](#createapikey) - Create an API Key
* [GetAPIKeys](#getapikeys) - List API Keys
* [DeleteAPIKey](#deleteapikey) - Delete an API Key

## CreateAPIKey

Creates a new API Key

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
    s := sdk.New()

    ctx := context.Background()
    res, err := s.APIKeys.CreateAPIKey(ctx, shared.APIKeysCreateParams{
        Name: "Johnnie Stamm",
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.APIKey != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                | Type                                                                     | Required                                                                 | Description                                                              |
| ------------------------------------------------------------------------ | ------------------------------------------------------------------------ | ------------------------------------------------------------------------ | ------------------------------------------------------------------------ |
| `ctx`                                                                    | [context.Context](https://pkg.go.dev/context#Context)                    | :heavy_check_mark:                                                       | The context to use for the request.                                      |
| `request`                                                                | [shared.APIKeysCreateParams](../../models/shared/apikeyscreateparams.md) | :heavy_check_mark:                                                       | The request object to use for the request.                               |


### Response

**[*operations.CreateAPIKeyResponse](../../models/operations/createapikeyresponse.md), error**


## GetAPIKeys

Returns a list of a user's API Keys.

### Example Usage

```go
package main

import(
	"context"
	"log"
	"openapi"
)

func main() {
    s := sdk.New()

    ctx := context.Background()
    res, err := s.APIKeys.GetAPIKeys(ctx)
    if err != nil {
        log.Fatal(err)
    }

    if res.APIKeys != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                             | Type                                                  | Required                                              | Description                                           |
| ----------------------------------------------------- | ----------------------------------------------------- | ----------------------------------------------------- | ----------------------------------------------------- |
| `ctx`                                                 | [context.Context](https://pkg.go.dev/context#Context) | :heavy_check_mark:                                    | The context to use for the request.                   |


### Response

**[*operations.GetAPIKeysResponse](../../models/operations/getapikeysresponse.md), error**


## DeleteAPIKey

Delete a specified API Key.

### Example Usage

```go
package main

import(
	"context"
	"log"
	"openapi"
	"openapi/pkg/models/operations"
)

func main() {
    s := sdk.New()

    ctx := context.Background()
    res, err := s.APIKeys.DeleteAPIKey(ctx, operations.DeleteAPIKeyRequest{
        APIKey: "deserunt",
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

| Parameter                                                                        | Type                                                                             | Required                                                                         | Description                                                                      |
| -------------------------------------------------------------------------------- | -------------------------------------------------------------------------------- | -------------------------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| `ctx`                                                                            | [context.Context](https://pkg.go.dev/context#Context)                            | :heavy_check_mark:                                                               | The context to use for the request.                                              |
| `request`                                                                        | [operations.DeleteAPIKeyRequest](../../models/operations/deleteapikeyrequest.md) | :heavy_check_mark:                                                               | The request object to use for the request.                                       |


### Response

**[*operations.DeleteAPIKeyResponse](../../models/operations/deleteapikeyresponse.md), error**

