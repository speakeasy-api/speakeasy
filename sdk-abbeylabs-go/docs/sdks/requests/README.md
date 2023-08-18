# Requests

## Overview

Requests are Access Requests that users make to get access to a resource.


<https://docs.abbey.io/getting-started/concepts#access-requests>
### Available Operations

* [CancelRequestByID](#cancelrequestbyid) - Cancel a Request by ID
* [CreateRequest](#createrequest) - Create a Request
* [GetRequestByID](#getrequestbyid) - Retrieve a Request by ID
* [ListRequests](#listrequests) - List Requests

## CancelRequestByID

Cancels the specified request.


### Example Usage

```go
package main

import(
	"context"
	"log"
	"openapi"
	"openapi/pkg/models/operations"
	"openapi/pkg/models/shared"
)

func main() {
    s := sdk.New()

    ctx := context.Background()
    res, err := s.Requests.CancelRequestByID(ctx, operations.CancelRequestByIDRequest{
        RequestCancelParams: shared.RequestCancelParams{
            Reason: sdk.String("modi"),
        },
        RequestID: "qui",
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.Request != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                                  | Type                                                                                       | Required                                                                                   | Description                                                                                |
| ------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------ |
| `ctx`                                                                                      | [context.Context](https://pkg.go.dev/context#Context)                                      | :heavy_check_mark:                                                                         | The context to use for the request.                                                        |
| `request`                                                                                  | [operations.CancelRequestByIDRequest](../../models/operations/cancelrequestbyidrequest.md) | :heavy_check_mark:                                                                         | The request object to use for the request.                                                 |


### Response

**[*operations.CancelRequestByIDResponse](../../models/operations/cancelrequestbyidresponse.md), error**


## CreateRequest

Creates a new request.

You will need to pass in a Grant Kit ID as the target of this request. This will create a request
against the latest version of the Grant Kit.

Grant Kit Versions are immutable and you won't be able to create a request against an older Grant Kit Version.
If you want to do this, you will have to roll forward by creating a new Grant Kit Version.


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
    res, err := s.Requests.CreateRequest(ctx, shared.RequestParams{
        GrantKitID: sdk.String("aliquid"),
        Reason: "cupiditate",
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.Request != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                                    | Type                                                         | Required                                                     | Description                                                  |
| ------------------------------------------------------------ | ------------------------------------------------------------ | ------------------------------------------------------------ | ------------------------------------------------------------ |
| `ctx`                                                        | [context.Context](https://pkg.go.dev/context#Context)        | :heavy_check_mark:                                           | The context to use for the request.                          |
| `request`                                                    | [shared.RequestParams](../../models/shared/requestparams.md) | :heavy_check_mark:                                           | The request object to use for the request.                   |


### Response

**[*operations.CreateRequestResponse](../../models/operations/createrequestresponse.md), error**


## GetRequestByID

Returns the details of a request.

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
    res, err := s.Requests.GetRequestByID(ctx, operations.GetRequestByIDRequest{
        RequestID: "quos",
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.Request != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                            | Type                                                                                 | Required                                                                             | Description                                                                          |
| ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------ |
| `ctx`                                                                                | [context.Context](https://pkg.go.dev/context#Context)                                | :heavy_check_mark:                                                                   | The context to use for the request.                                                  |
| `request`                                                                            | [operations.GetRequestByIDRequest](../../models/operations/getrequestbyidrequest.md) | :heavy_check_mark:                                                                   | The request object to use for the request.                                           |


### Response

**[*operations.GetRequestByIDResponse](../../models/operations/getrequestbyidresponse.md), error**


## ListRequests

Returns a list of requests.

Requests are sorted by creation date, descending.


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
    res, err := s.Requests.ListRequests(ctx)
    if err != nil {
        log.Fatal(err)
    }

    if res.Requests != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                             | Type                                                  | Required                                              | Description                                           |
| ----------------------------------------------------- | ----------------------------------------------------- | ----------------------------------------------------- | ----------------------------------------------------- |
| `ctx`                                                 | [context.Context](https://pkg.go.dev/context#Context) | :heavy_check_mark:                                    | The context to use for the request.                   |


### Response

**[*operations.ListRequestsResponse](../../models/operations/listrequestsresponse.md), error**

