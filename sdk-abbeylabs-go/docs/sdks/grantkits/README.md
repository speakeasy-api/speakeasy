# GrantKits

## Overview

Grant Kits are what you configure in code to control and automatically right-size permissions for resources.
A Grant Kit has 3 components:

1. Workflow to configure how someone should get access.
2. Policies to configure if someone should get access.
3. Output to configure how and where Grants should materialize.


<https://docs.abbey.io/getting-started/concepts#grant-kits>
### Available Operations

* [CreateGrantKit](#creategrantkit) - Create a Grant Kit
* [GetGrantKits](#getgrantkits) - List Grant Kits
* [DeleteGrantKit](#deletegrantkit) - Delete a Grant Kit
* [GetGrantKitByID](#getgrantkitbyid) - Retrieve a Grant Kit by ID
* [ListGrantKitVersionsByID](#listgrantkitversionsbyid) - List Grant Kit Versions of a Grant Kit ID
* [UpdateGrantKit](#updategrantkit) - Update a Grant Kit

## CreateGrantKit

Creates a new Grant Kit

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
    res, err := s.GrantKits.CreateGrantKit(ctx, shared.GrantKitCreateParams{
        Description: "molestiae",
        Name: "Irving Lehner",
        Output: shared.Output{
            Append: sdk.String("nisi"),
            Location: "recusandae",
            Overwrite: sdk.String("temporibus"),
        },
        Policies: &shared.Policies{
            GrantIf: []shared.Policy{
                shared.Policy{
                    Bundle: sdk.String("quis"),
                    Query: sdk.String("veritatis"),
                },
            },
            RevokeIf: []shared.Policy{
                shared.Policy{
                    Bundle: sdk.String("perferendis"),
                    Query: sdk.String("ipsam"),
                },
                shared.Policy{
                    Bundle: sdk.String("repellendus"),
                    Query: sdk.String("sapiente"),
                },
                shared.Policy{
                    Bundle: sdk.String("quo"),
                    Query: sdk.String("odit"),
                },
            },
        },
        Workflow: &shared.GrantWorkflow{
            Steps: []shared.Step{
                shared.Step{
                    Reviewers: shared.Reviewers{
                        AllOf: []string{
                            "maiores",
                            "molestiae",
                            "quod",
                            "quod",
                        },
                        OneOf: []string{
                            "totam",
                            "porro",
                        },
                    },
                    SkipIf: []shared.Policy{
                        shared.Policy{
                            Bundle: sdk.String("dicta"),
                            Query: sdk.String("nam"),
                        },
                        shared.Policy{
                            Bundle: sdk.String("officia"),
                            Query: sdk.String("occaecati"),
                        },
                        shared.Policy{
                            Bundle: sdk.String("fugit"),
                            Query: sdk.String("deleniti"),
                        },
                    },
                },
                shared.Step{
                    Reviewers: shared.Reviewers{
                        AllOf: []string{
                            "optio",
                            "totam",
                            "beatae",
                            "commodi",
                        },
                        OneOf: []string{
                            "modi",
                            "qui",
                        },
                    },
                    SkipIf: []shared.Policy{
                        shared.Policy{
                            Bundle: sdk.String("cum"),
                            Query: sdk.String("esse"),
                        },
                        shared.Policy{
                            Bundle: sdk.String("ipsum"),
                            Query: sdk.String("excepturi"),
                        },
                        shared.Policy{
                            Bundle: sdk.String("aspernatur"),
                            Query: sdk.String("perferendis"),
                        },
                        shared.Policy{
                            Bundle: sdk.String("ad"),
                            Query: sdk.String("natus"),
                        },
                    },
                },
                shared.Step{
                    Reviewers: shared.Reviewers{
                        AllOf: []string{
                            "iste",
                        },
                        OneOf: []string{
                            "natus",
                        },
                    },
                    SkipIf: []shared.Policy{
                        shared.Policy{
                            Bundle: sdk.String("hic"),
                            Query: sdk.String("saepe"),
                        },
                        shared.Policy{
                            Bundle: sdk.String("fuga"),
                            Query: sdk.String("in"),
                        },
                    },
                },
                shared.Step{
                    Reviewers: shared.Reviewers{
                        AllOf: []string{
                            "iste",
                            "iure",
                        },
                        OneOf: []string{
                            "quidem",
                            "architecto",
                            "ipsa",
                            "reiciendis",
                        },
                    },
                    SkipIf: []shared.Policy{
                        shared.Policy{
                            Bundle: sdk.String("mollitia"),
                            Query: sdk.String("laborum"),
                        },
                        shared.Policy{
                            Bundle: sdk.String("dolores"),
                            Query: sdk.String("dolorem"),
                        },
                        shared.Policy{
                            Bundle: sdk.String("corporis"),
                            Query: sdk.String("explicabo"),
                        },
                    },
                },
            },
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.GrantKit != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                  | Type                                                                       | Required                                                                   | Description                                                                |
| -------------------------------------------------------------------------- | -------------------------------------------------------------------------- | -------------------------------------------------------------------------- | -------------------------------------------------------------------------- |
| `ctx`                                                                      | [context.Context](https://pkg.go.dev/context#Context)                      | :heavy_check_mark:                                                         | The context to use for the request.                                        |
| `request`                                                                  | [shared.GrantKitCreateParams](../../models/shared/grantkitcreateparams.md) | :heavy_check_mark:                                                         | The request object to use for the request.                                 |


### Response

**[*operations.CreateGrantKitResponse](../../models/operations/creategrantkitresponse.md), error**


## GetGrantKits

Returns a list of the latest versions of each grant kit in the organization.

Grant Kits are sorted by creation date, descending.


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
    res, err := s.GrantKits.GetGrantKits(ctx)
    if err != nil {
        log.Fatal(err)
    }

    if res.GrantKits != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                             | Type                                                  | Required                                              | Description                                           |
| ----------------------------------------------------- | ----------------------------------------------------- | ----------------------------------------------------- | ----------------------------------------------------- |
| `ctx`                                                 | [context.Context](https://pkg.go.dev/context#Context) | :heavy_check_mark:                                    | The context to use for the request.                   |


### Response

**[*operations.GetGrantKitsResponse](../../models/operations/getgrantkitsresponse.md), error**


## DeleteGrantKit

Deletes the specified grant kit.

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
    res, err := s.GrantKits.DeleteGrantKit(ctx, operations.DeleteGrantKitRequest{
        GrantKitIDOrName: "nobis",
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.GrantKit != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                            | Type                                                                                 | Required                                                                             | Description                                                                          |
| ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------ |
| `ctx`                                                                                | [context.Context](https://pkg.go.dev/context#Context)                                | :heavy_check_mark:                                                                   | The context to use for the request.                                                  |
| `request`                                                                            | [operations.DeleteGrantKitRequest](../../models/operations/deletegrantkitrequest.md) | :heavy_check_mark:                                                                   | The request object to use for the request.                                           |


### Response

**[*operations.DeleteGrantKitResponse](../../models/operations/deletegrantkitresponse.md), error**


## GetGrantKitByID

Returns the details of a Grant Kit.

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
    res, err := s.GrantKits.GetGrantKitByID(ctx, operations.GetGrantKitByIDRequest{
        GrantKitIDOrName: "enim",
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.GrantKit != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                              | Type                                                                                   | Required                                                                               | Description                                                                            |
| -------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------- |
| `ctx`                                                                                  | [context.Context](https://pkg.go.dev/context#Context)                                  | :heavy_check_mark:                                                                     | The context to use for the request.                                                    |
| `request`                                                                              | [operations.GetGrantKitByIDRequest](../../models/operations/getgrantkitbyidrequest.md) | :heavy_check_mark:                                                                     | The request object to use for the request.                                             |


### Response

**[*operations.GetGrantKitByIDResponse](../../models/operations/getgrantkitbyidresponse.md), error**


## ListGrantKitVersionsByID

Returns all versions of a grant kit.

Grant Kits are sorted by creation date, descending.


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
    res, err := s.GrantKits.ListGrantKitVersionsByID(ctx, operations.ListGrantKitVersionsByIDRequest{
        GrantKitIDOrName: "omnis",
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.GrantKitVersions != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                                                | Type                                                                                                     | Required                                                                                                 | Description                                                                                              |
| -------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- |
| `ctx`                                                                                                    | [context.Context](https://pkg.go.dev/context#Context)                                                    | :heavy_check_mark:                                                                                       | The context to use for the request.                                                                      |
| `request`                                                                                                | [operations.ListGrantKitVersionsByIDRequest](../../models/operations/listgrantkitversionsbyidrequest.md) | :heavy_check_mark:                                                                                       | The request object to use for the request.                                                               |


### Response

**[*operations.ListGrantKitVersionsByIDResponse](../../models/operations/listgrantkitversionsbyidresponse.md), error**


## UpdateGrantKit

Updates the specified grant kit.


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
    res, err := s.GrantKits.UpdateGrantKit(ctx, operations.UpdateGrantKitRequest{
        GrantKitUpdateParams: shared.GrantKitUpdateParams{
            Description: "nemo",
            Name: "Velma Batz",
            Output: shared.Output{
                Append: sdk.String("doloribus"),
                Location: "sapiente",
                Overwrite: sdk.String("architecto"),
            },
            Policies: &shared.Policies{
                GrantIf: []shared.Policy{
                    shared.Policy{
                        Bundle: sdk.String("dolorem"),
                        Query: sdk.String("culpa"),
                    },
                    shared.Policy{
                        Bundle: sdk.String("consequuntur"),
                        Query: sdk.String("repellat"),
                    },
                    shared.Policy{
                        Bundle: sdk.String("mollitia"),
                        Query: sdk.String("occaecati"),
                    },
                },
                RevokeIf: []shared.Policy{
                    shared.Policy{
                        Bundle: sdk.String("commodi"),
                        Query: sdk.String("quam"),
                    },
                    shared.Policy{
                        Bundle: sdk.String("molestiae"),
                        Query: sdk.String("velit"),
                    },
                },
            },
            Workflow: &shared.GrantWorkflow{
                Steps: []shared.Step{
                    shared.Step{
                        Reviewers: shared.Reviewers{
                            AllOf: []string{
                                "quis",
                            },
                            OneOf: []string{
                                "laborum",
                            },
                        },
                        SkipIf: []shared.Policy{
                            shared.Policy{
                                Bundle: sdk.String("enim"),
                                Query: sdk.String("odit"),
                            },
                            shared.Policy{
                                Bundle: sdk.String("quo"),
                                Query: sdk.String("sequi"),
                            },
                            shared.Policy{
                                Bundle: sdk.String("tenetur"),
                                Query: sdk.String("ipsam"),
                            },
                        },
                    },
                    shared.Step{
                        Reviewers: shared.Reviewers{
                            AllOf: []string{
                                "possimus",
                                "aut",
                                "quasi",
                            },
                            OneOf: []string{
                                "temporibus",
                                "laborum",
                                "quasi",
                            },
                        },
                        SkipIf: []shared.Policy{
                            shared.Policy{
                                Bundle: sdk.String("voluptatibus"),
                                Query: sdk.String("vero"),
                            },
                            shared.Policy{
                                Bundle: sdk.String("nihil"),
                                Query: sdk.String("praesentium"),
                            },
                            shared.Policy{
                                Bundle: sdk.String("voluptatibus"),
                                Query: sdk.String("ipsa"),
                            },
                            shared.Policy{
                                Bundle: sdk.String("omnis"),
                                Query: sdk.String("voluptate"),
                            },
                        },
                    },
                    shared.Step{
                        Reviewers: shared.Reviewers{
                            AllOf: []string{
                                "perferendis",
                                "doloremque",
                                "reprehenderit",
                            },
                            OneOf: []string{
                                "maiores",
                                "dicta",
                            },
                        },
                        SkipIf: []shared.Policy{
                            shared.Policy{
                                Bundle: sdk.String("dolore"),
                                Query: sdk.String("iusto"),
                            },
                            shared.Policy{
                                Bundle: sdk.String("dicta"),
                                Query: sdk.String("harum"),
                            },
                        },
                    },
                },
            },
        },
        GrantKitIDOrName: "enim",
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.GrantKit != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                            | Type                                                                                 | Required                                                                             | Description                                                                          |
| ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------ |
| `ctx`                                                                                | [context.Context](https://pkg.go.dev/context#Context)                                | :heavy_check_mark:                                                                   | The context to use for the request.                                                  |
| `request`                                                                            | [operations.UpdateGrantKitRequest](../../models/operations/updategrantkitrequest.md) | :heavy_check_mark:                                                                   | The request object to use for the request.                                           |


### Response

**[*operations.UpdateGrantKitResponse](../../models/operations/updategrantkitresponse.md), error**

