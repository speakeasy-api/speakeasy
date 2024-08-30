# Source
(*Source*)

## Overview

### Available Operations

* [Get](#get) - Get Source
* [Update](#update) - Update Source

## Get

Retrieve the source information from the workflow file, before and after applying the studio modifications overlay.

### Example Usage

```go
package main

import(
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk"
	"context"
	"log"
)

func main() {
    s := sdk.New(
        sdk.WithSecurity("<YOUR_API_KEY_HERE>"),
    )

    ctx := context.Background()
    res, err := s.Source.Get(ctx)
    if err != nil {
        log.Fatal(err)
    }
    if res.SourceResponse != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                                | Type                                                     | Required                                                 | Description                                              |
| -------------------------------------------------------- | -------------------------------------------------------- | -------------------------------------------------------- | -------------------------------------------------------- |
| `ctx`                                                    | [context.Context](https://pkg.go.dev/context#Context)    | :heavy_check_mark:                                       | The context to use for the request.                      |
| `opts`                                                   | [][operations.Option](../../models/operations/option.md) | :heavy_minus_sign:                                       | The options for this request.                            |

### Response

**[*operations.GetSourceResponse](../../models/operations/getsourceresponse.md), error**

### Errors

| Error Object       | Status Code        | Content Type       |
| ------------------ | ------------------ | ------------------ |
| sdkerrors.SDKError | 4xx-5xx            | */*                |


## Update

Update the source with studio modifications overlay contents. This will re-run the source in the workflow.

### Example Usage

```go
package main

import(
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk/models/operations"
	"context"
	"log"
)

func main() {
    s := sdk.New(
        sdk.WithSecurity("<YOUR_API_KEY_HERE>"),
    )
    request := operations.UpdateSourceRequestBody{}
    ctx := context.Background()
    res, err := s.Source.Update(ctx, request)
    if err != nil {
        log.Fatal(err)
    }
    if res.SourceResponse != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                                | Type                                                                                     | Required                                                                                 | Description                                                                              |
| ---------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| `ctx`                                                                                    | [context.Context](https://pkg.go.dev/context#Context)                                    | :heavy_check_mark:                                                                       | The context to use for the request.                                                      |
| `request`                                                                                | [operations.UpdateSourceRequestBody](../../models/operations/updatesourcerequestbody.md) | :heavy_check_mark:                                                                       | The request object to use for the request.                                               |
| `opts`                                                                                   | [][operations.Option](../../models/operations/option.md)                                 | :heavy_minus_sign:                                                                       | The options for this request.                                                            |

### Response

**[*operations.UpdateSourceResponse](../../models/operations/updatesourceresponse.md), error**

### Errors

| Error Object       | Status Code        | Content Type       |
| ------------------ | ------------------ | ------------------ |
| sdkerrors.SDKError | 4xx-5xx            | */*                |
