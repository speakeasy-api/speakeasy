# Grants

## Overview

Grants are permissions that reflect the result of an access request going through the process of evaluating 
policies and approval workflows where all approval conditions are met.

Grants may be revoked manually by a user or automatically if a time-based or attribute-based policy is
included in the corresponding Grant Kit's policy.


<https://docs.abbey.io/getting-started/concepts#grants>
### Available Operations

* [ListGrants](#listgrants) - List Grants
* [GetGrantByID](#getgrantbyid) - Retrieve a Grant by ID
* [RevokeGrant](#revokegrant) - Revoke a Grant by ID

## ListGrants

Returns a list of all the grants belonging to a user.

Grants are sorted by creation date, descending. Creation date effectively means when the grant was approved.


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
    res, err := s.Grants.ListGrants(ctx)
    if err != nil {
        log.Fatal(err)
    }

    if res.Grants != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                             | Type                                                  | Required                                              | Description                                           |
| ----------------------------------------------------- | ----------------------------------------------------- | ----------------------------------------------------- | ----------------------------------------------------- |
| `ctx`                                                 | [context.Context](https://pkg.go.dev/context#Context) | :heavy_check_mark:                                    | The context to use for the request.                   |


### Response

**[*operations.ListGrantsResponse](../../models/operations/listgrantsresponse.md), error**


## GetGrantByID

Returns the details of a grant.


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
    res, err := s.Grants.GetGrantByID(ctx, operations.GetGrantByIDRequest{
        GrantID: "accusamus",
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.Grant != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                        | Type                                                                             | Required                                                                         | Description                                                                      |
| -------------------------------------------------------------------------------- | -------------------------------------------------------------------------------- | -------------------------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| `ctx`                                                                            | [context.Context](https://pkg.go.dev/context#Context)                            | :heavy_check_mark:                                                               | The context to use for the request.                                              |
| `request`                                                                        | [operations.GetGrantByIDRequest](../../models/operations/getgrantbyidrequest.md) | :heavy_check_mark:                                                               | The request object to use for the request.                                       |


### Response

**[*operations.GetGrantByIDResponse](../../models/operations/getgrantbyidresponse.md), error**


## RevokeGrant

Revokes the specified grant.


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
    res, err := s.Grants.RevokeGrant(ctx, operations.RevokeGrantRequest{
        GrantID: "commodi",
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.Grant != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                      | Type                                                                           | Required                                                                       | Description                                                                    |
| ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------ |
| `ctx`                                                                          | [context.Context](https://pkg.go.dev/context#Context)                          | :heavy_check_mark:                                                             | The context to use for the request.                                            |
| `request`                                                                      | [operations.RevokeGrantRequest](../../models/operations/revokegrantrequest.md) | :heavy_check_mark:                                                             | The request object to use for the request.                                     |


### Response

**[*operations.RevokeGrantResponse](../../models/operations/revokegrantresponse.md), error**

