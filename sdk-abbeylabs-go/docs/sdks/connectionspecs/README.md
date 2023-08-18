# ConnectionSpecs

## Overview

Connection Specs are the templates for creating connections.
They are used to validate connection parameters and to provide a UI for creating connections.


<https://docs.abbey.io>
### Available Operations

* [ListConnectionSpecs](#listconnectionspecs) - List Connection Specs

## ListConnectionSpecs

Returns a list of connection specs.
The connection specs are returned sorted alphabetically.


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
    res, err := s.ConnectionSpecs.ListConnectionSpecs(ctx)
    if err != nil {
        log.Fatal(err)
    }

    if res.ConnectionSpecListing != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                             | Type                                                  | Required                                              | Description                                           |
| ----------------------------------------------------- | ----------------------------------------------------- | ----------------------------------------------------- | ----------------------------------------------------- |
| `ctx`                                                 | [context.Context](https://pkg.go.dev/context#Context) | :heavy_check_mark:                                    | The context to use for the request.                   |


### Response

**[*operations.ListConnectionSpecsResponse](../../models/operations/listconnectionspecsresponse.md), error**

