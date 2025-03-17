# SDK

## Overview

### Available Operations

* [GenerateOverlay](#generateoverlay) - Generate Overlay
* [Exit](#exit) - Exit

## GenerateOverlay

Generate an overlay from two yaml files

### Example Usage

```go
package main

import(
	"context"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk/models/components"
	"log"
)

func main() {
    ctx := context.Background()

    s := sdk.New(
        sdk.WithSecurity("<YOUR_API_KEY_HERE>"),
    )

    res, err := s.GenerateOverlay(ctx, components.OverlayCompareRequestBody{
        Before: "<value>",
        After: "<value>",
    })
    if err != nil {
        log.Fatal(err)
    }
    if res.OverlayCompareResponse != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                                    | Type                                                                                         | Required                                                                                     | Description                                                                                  |
| -------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| `ctx`                                                                                        | [context.Context](https://pkg.go.dev/context#Context)                                        | :heavy_check_mark:                                                                           | The context to use for the request.                                                          |
| `request`                                                                                    | [components.OverlayCompareRequestBody](../../models/components/overlaycomparerequestbody.md) | :heavy_check_mark:                                                                           | The request object to use for the request.                                                   |
| `opts`                                                                                       | [][operations.Option](../../models/operations/option.md)                                     | :heavy_minus_sign:                                                                           | The options for this request.                                                                |

### Response

**[*operations.GenerateOverlayResponse](../../models/operations/generateoverlayresponse.md), error**

### Errors

| Error Type         | Status Code        | Content Type       |
| ------------------ | ------------------ | ------------------ |
| sdkerrors.SDKError | 4XX, 5XX           | \*/\*              |

## Exit

Exit the CLI

### Example Usage

```go
package main

import(
	"context"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk"
	"log"
)

func main() {
    ctx := context.Background()

    s := sdk.New(
        sdk.WithSecurity("<YOUR_API_KEY_HERE>"),
    )

    res, err := s.Exit(ctx)
    if err != nil {
        log.Fatal(err)
    }
    if res != nil {
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

**[*operations.ExitResponse](../../models/operations/exitresponse.md), error**

### Errors

| Error Type         | Status Code        | Content Type       |
| ------------------ | ------------------ | ------------------ |
| sdkerrors.SDKError | 4XX, 5XX           | \*/\*              |