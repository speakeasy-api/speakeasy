# SpeakeasyStudio SDK


## Overview

### Available Operations

* [CheckHealth](#checkhealth) - Health Check
* [Run](#run) - Run
* [GetSource](#getsource) - Get Source
* [UpdateSource](#updatesource) - Update Source
* [FileChanges](#filechanges) - File Changes

## CheckHealth

Check the CLI health and return relevant information.

### Example Usage

```go
package main

import(
	generatedstudiosdk "github.com/speakeasy-api/speakeasy/internal/run/generated-studio-sdk"
	"context"
	"log"
)

func main() {
    s := generatedstudiosdk.New()

    ctx := context.Background()
    res, err := s.CheckHealth(ctx)
    if err != nil {
        log.Fatal(err)
    }
    if res.HealthResponse != nil {
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

**[*operations.CheckHealthResponse](../../models/operations/checkhealthresponse.md), error**
| Error Object       | Status Code        | Content Type       |
| ------------------ | ------------------ | ------------------ |
| sdkerrors.SDKError | 4xx-5xx            | */*                |

## Run

Regenerate the currently selected targets.

### Example Usage

```go
package main

import(
	generatedstudiosdk "github.com/speakeasy-api/speakeasy/internal/run/generated-studio-sdk"
	"context"
	"log"
)

func main() {
    s := generatedstudiosdk.New()

    ctx := context.Background()
    res, err := s.Run(ctx)
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

**[*operations.RunResponse](../../models/operations/runresponse.md), error**
| Error Object       | Status Code        | Content Type       |
| ------------------ | ------------------ | ------------------ |
| sdkerrors.SDKError | 4xx-5xx            | */*                |

## GetSource

Retrieve the source information from the workflow file, before and after applying the studio modifications overlay.

### Example Usage

```go
package main

import(
	generatedstudiosdk "github.com/speakeasy-api/speakeasy/internal/run/generated-studio-sdk"
	"context"
	"log"
)

func main() {
    s := generatedstudiosdk.New()

    ctx := context.Background()
    res, err := s.GetSource(ctx)
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
| Error Object       | Status Code        | Content Type       |
| ------------------ | ------------------ | ------------------ |
| sdkerrors.SDKError | 4xx-5xx            | */*                |

## UpdateSource

Update the source with studio modifications overlay contents. This will re-run the source in the workflow.

### Example Usage

```go
package main

import(
	generatedstudiosdk "github.com/speakeasy-api/speakeasy/internal/run/generated-studio-sdk"
	"github.com/speakeasy-api/speakeasy/internal/run/generated-studio-sdk/models/operations"
	"context"
	"log"
)

func main() {
    s := generatedstudiosdk.New()
    request := operations.UpdateSourceRequestBody{
        Overlay: "<value>",
    }
    ctx := context.Background()
    res, err := s.UpdateSource(ctx, request)
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
| Error Object       | Status Code        | Content Type       |
| ------------------ | ------------------ | ------------------ |
| sdkerrors.SDKError | 4xx-5xx            | */*                |

## FileChanges

SSE endpoint to send changes detected on the local file system.

### Example Usage

```go
package main

import(
	generatedstudiosdk "github.com/speakeasy-api/speakeasy/internal/run/generated-studio-sdk"
	"context"
	"log"
)

func main() {
    s := generatedstudiosdk.New()

    ctx := context.Background()
    res, err := s.FileChanges(ctx)
    if err != nil {
        log.Fatal(err)
    }
    if res.Res != nil {
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

**[*operations.FileChangesResponse](../../models/operations/filechangesresponse.md), error**
| Error Object       | Status Code        | Content Type       |
| ------------------ | ------------------ | ------------------ |
| sdkerrors.SDKError | 4xx-5xx            | */*                |
