# Identities

## Overview

User metadata used for enriching data.
Enriched data is used to write richer policies, workflows, and outputs.


<https://docs.abbey.io>
### Available Operations

* [CreateIdentity](#createidentity) - Create an Identity
* [DeleteIdentity](#deleteidentity) - Delete an Identity
* [GetIdentity](#getidentity) - Retrieve an Identity

## CreateIdentity

Creates a new identity.

An identity represents a human, service, or workload.


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
    res, err := s.Identities.CreateIdentity(ctx, shared.IdentityParams{
        Linked: map[string][]interface{}{
            "quae": []interface{}{
                "quidem",
            },
            "molestias": []interface{}{
                "pariatur",
                "modi",
                "praesentium",
            },
            "rem": []interface{}{
                "quasi",
                "repudiandae",
                "sint",
                "veritatis",
            },
            "itaque": []interface{}{
                "enim",
                "consequatur",
            },
        },
        Name: "Taylor Cole",
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.Identity != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                                      | Type                                                           | Required                                                       | Description                                                    |
| -------------------------------------------------------------- | -------------------------------------------------------------- | -------------------------------------------------------------- | -------------------------------------------------------------- |
| `ctx`                                                          | [context.Context](https://pkg.go.dev/context#Context)          | :heavy_check_mark:                                             | The context to use for the request.                            |
| `request`                                                      | [shared.IdentityParams](../../models/shared/identityparams.md) | :heavy_check_mark:                                             | The request object to use for the request.                     |


### Response

**[*operations.CreateIdentityResponse](../../models/operations/createidentityresponse.md), error**


## DeleteIdentity

Deletes the specified identity.

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
    res, err := s.Identities.DeleteIdentity(ctx, operations.DeleteIdentityRequest{
        IdentityID: "quibusdam",
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.StatusCode == http.StatusOK {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                            | Type                                                                                 | Required                                                                             | Description                                                                          |
| ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------ |
| `ctx`                                                                                | [context.Context](https://pkg.go.dev/context#Context)                                | :heavy_check_mark:                                                                   | The context to use for the request.                                                  |
| `request`                                                                            | [operations.DeleteIdentityRequest](../../models/operations/deleteidentityrequest.md) | :heavy_check_mark:                                                                   | The request object to use for the request.                                           |


### Response

**[*operations.DeleteIdentityResponse](../../models/operations/deleteidentityresponse.md), error**


## GetIdentity

Returns the details of an identity.

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
    res, err := s.Identities.GetIdentity(ctx, operations.GetIdentityRequest{
        IdentityID: "labore",
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.Identity != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                      | Type                                                                           | Required                                                                       | Description                                                                    |
| ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------ |
| `ctx`                                                                          | [context.Context](https://pkg.go.dev/context#Context)                          | :heavy_check_mark:                                                             | The context to use for the request.                                            |
| `request`                                                                      | [operations.GetIdentityRequest](../../models/operations/getidentityrequest.md) | :heavy_check_mark:                                                             | The request object to use for the request.                                     |


### Response

**[*operations.GetIdentityResponse](../../models/operations/getidentityresponse.md), error**

