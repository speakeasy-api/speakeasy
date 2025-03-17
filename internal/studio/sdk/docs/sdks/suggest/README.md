# Suggest
(*Suggest*)

## Overview

### Available Operations

* [MethodNames](#methodnames) - Suggest Method Names

## MethodNames

Suggest method names for the current source.

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

    res, err := s.Suggest.MethodNames(ctx)
    if err != nil {
        log.Fatal(err)
    }
    if res.SuggestResponse != nil {
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

**[*operations.SuggestMethodNamesResponse](../../models/operations/suggestmethodnamesresponse.md), error**

### Errors

| Error Type         | Status Code        | Content Type       |
| ------------------ | ------------------ | ------------------ |
| sdkerrors.SDKError | 4XX, 5XX           | \*/\*              |