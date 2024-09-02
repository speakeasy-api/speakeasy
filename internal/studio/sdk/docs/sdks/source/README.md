# Source
(*Source*)

## Overview

### Available Operations

* [SuggestMethodNames](#suggestmethodnames) - Suggest Method Names

## SuggestMethodNames

Suggest method names for the current source.

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
    res, err := s.Source.SuggestMethodNames(ctx)
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

| Error Object       | Status Code        | Content Type       |
| ------------------ | ------------------ | ------------------ |
| sdkerrors.SDKError | 4xx-5xx            | */*                |
