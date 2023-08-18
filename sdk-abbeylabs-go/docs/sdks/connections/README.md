# Connections

## Overview

Connections are authenticated, with scopes if available, and made available to Abbey Grant Kits at runtime.


<https://docs.abbey.io>
### Available Operations

* [CreateConnection](#createconnection) - Create a Connection
* [GetConnection](#getconnection) - Retrieve a Connection by ID
* [ListConnections](#listconnections) - List Connections
* [UpdateConnection](#updateconnection) - Update a Connection

## CreateConnection

Creates a new connection.

Connections are authenticated, with scopes if available, and made available to Abbey Grant Kits at runtime.


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
    res, err := s.Connections.CreateConnection(ctx, shared.ConnectionParams{})
    if err != nil {
        log.Fatal(err)
    }

    if res.Connection != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                                          | Type                                                               | Required                                                           | Description                                                        |
| ------------------------------------------------------------------ | ------------------------------------------------------------------ | ------------------------------------------------------------------ | ------------------------------------------------------------------ |
| `ctx`                                                              | [context.Context](https://pkg.go.dev/context#Context)              | :heavy_check_mark:                                                 | The context to use for the request.                                |
| `request`                                                          | [shared.ConnectionParams](../../models/shared/connectionparams.md) | :heavy_check_mark:                                                 | The request object to use for the request.                         |


### Response

**[*operations.CreateConnectionResponse](../../models/operations/createconnectionresponse.md), error**


## GetConnection

Returns the details of a connection.

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
    res, err := s.Connections.GetConnection(ctx, operations.GetConnectionRequest{
        ConnectionID: "suscipit",
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.Connection != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                          | Type                                                                               | Required                                                                           | Description                                                                        |
| ---------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| `ctx`                                                                              | [context.Context](https://pkg.go.dev/context#Context)                              | :heavy_check_mark:                                                                 | The context to use for the request.                                                |
| `request`                                                                          | [operations.GetConnectionRequest](../../models/operations/getconnectionrequest.md) | :heavy_check_mark:                                                                 | The request object to use for the request.                                         |


### Response

**[*operations.GetConnectionResponse](../../models/operations/getconnectionresponse.md), error**


## ListConnections

Returns a list of connections.
The connections are returned sorted by creation date, descending.


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
    res, err := s.Connections.ListConnections(ctx)
    if err != nil {
        log.Fatal(err)
    }

    if res.ConnectionListing != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                             | Type                                                  | Required                                              | Description                                           |
| ----------------------------------------------------- | ----------------------------------------------------- | ----------------------------------------------------- | ----------------------------------------------------- |
| `ctx`                                                 | [context.Context](https://pkg.go.dev/context#Context) | :heavy_check_mark:                                    | The context to use for the request.                   |


### Response

**[*operations.ListConnectionsResponse](../../models/operations/listconnectionsresponse.md), error**


## UpdateConnection

Updates the specified connection.


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
    res, err := s.Connections.UpdateConnection(ctx, operations.UpdateConnectionRequest{
        ConnectionUpdateParams: shared.ConnectionUpdateParams{
            Name: sdk.String("Dr. Valerie Toy"),
        },
        ConnectionID: "suscipit",
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.Connection != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                                | Type                                                                                     | Required                                                                                 | Description                                                                              |
| ---------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| `ctx`                                                                                    | [context.Context](https://pkg.go.dev/context#Context)                                    | :heavy_check_mark:                                                                       | The context to use for the request.                                                      |
| `request`                                                                                | [operations.UpdateConnectionRequest](../../models/operations/updateconnectionrequest.md) | :heavy_check_mark:                                                                       | The request object to use for the request.                                               |


### Response

**[*operations.UpdateConnectionResponse](../../models/operations/updateconnectionresponse.md), error**

