# Run
(*Run*)

## Overview

### Available Operations

* [GetLastOutput](#getlastoutput) - Run
* [RegenerateTargets](#regeneratetargets) - Run

## GetLastOutput

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
    res, err := s.Run.GetLastOutput(ctx)
    if err != nil {
        log.Fatal(err)
    }
    if res.RunResponse != nil {
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

**[*operations.GetRunResponse](../../models/operations/getrunresponse.md), error**

### Errors

| Error Object       | Status Code        | Content Type       |
| ------------------ | ------------------ | ------------------ |
| sdkerrors.SDKError | 4xx-5xx            | */*                |


## RegenerateTargets

Regenerate the currently selected targets.

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
    request := operations.RunRequestBody{}
    ctx := context.Background()
    res, err := s.Run.RegenerateTargets(ctx, request)
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

| Error Object       | Status Code        | Content Type       |
| ------------------ | ------------------ | ------------------ |
| sdkerrors.SDKError | 4xx-5xx            | */*                |
