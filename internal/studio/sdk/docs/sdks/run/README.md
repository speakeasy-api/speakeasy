# Run
(*Run*)

## Overview

### Available Operations

* [GetLastResult](#getlastresult) - Run
* [ReRun](#rerun) - Run

## GetLastResult

Get the output of the last run.

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
    res, err := s.Run.GetLastResult(ctx)
    if err != nil {
        log.Fatal(err)
    }
    if res.RunResponseStreamEvent != nil {
        defer res.RunResponseStreamEvent.Close()

        for res.RunResponseStreamEvent.Next() {
            event := res.RunResponseStreamEvent.Value()
            log.Print(event)
            // Handle the event
	      }
    }
}
```

### Parameters

| Parameter                                                | Type                                                     | Required                                                 | Description                                              |
| -------------------------------------------------------- | -------------------------------------------------------- | -------------------------------------------------------- | -------------------------------------------------------- |
| `ctx`                                                    | [context.Context](https://pkg.go.dev/context#Context)    | :heavy_check_mark:                                       | The context to use for the request.                      |
| `opts`                                                   | [][operations.Option](../../models/operations/option.md) | :heavy_minus_sign:                                       | The options for this request.                            |

### Response

**[*operations.GetRunResponse](../../models/operations/getrunresponse.md), error**

### Errors

| Error Type         | Status Code        | Content Type       |
| ------------------ | ------------------ | ------------------ |
| sdkerrors.SDKError | 4XX, 5XX           | \*/\*              |

## ReRun

Regenerate the currently selected targets.

### Example Usage

```go
package main

import(
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk"
	"context"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk/models/operations"
	"log"
)

func main() {
    s := sdk.New(
        sdk.WithSecurity("<YOUR_API_KEY_HERE>"),
    )

    ctx := context.Background()
    res, err := s.Run.ReRun(ctx, operations.RunRequestBody{})
    if err != nil {
        log.Fatal(err)
    }
    if res.OneOf != nil {
        defer res.OneOf.Close()

        for res.OneOf.Next() {
            event := res.OneOf.Value()
            log.Print(event)
            // Handle the event
	      }
    }
}
```

### Parameters

| Parameter                                                              | Type                                                                   | Required                                                               | Description                                                            |
| ---------------------------------------------------------------------- | ---------------------------------------------------------------------- | ---------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| `ctx`                                                                  | [context.Context](https://pkg.go.dev/context#Context)                  | :heavy_check_mark:                                                     | The context to use for the request.                                    |
| `request`                                                              | [operations.RunRequestBody](../../models/operations/runrequestbody.md) | :heavy_check_mark:                                                     | The request object to use for the request.                             |
| `opts`                                                                 | [][operations.Option](../../models/operations/option.md)               | :heavy_minus_sign:                                                     | The options for this request.                                          |

### Response

**[*operations.RunResponse](../../models/operations/runresponse.md), error**

### Errors

| Error Type         | Status Code        | Content Type       |
| ------------------ | ------------------ | ------------------ |
| sdkerrors.SDKError | 4XX, 5XX           | \*/\*              |