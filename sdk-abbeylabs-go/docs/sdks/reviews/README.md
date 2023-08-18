# Reviews

## Overview

Reviews are decisions made by a reviewer on an Access Request.

A Reviewer might approve or deny a request.


<https://docs.abbey.io/product/approving-or-denying-access-requests>
### Available Operations

* [ListReviews](#listreviews) - List Reviews
* [ApproveReview](#approvereview) - Approve a Review
* [DenyReview](#denyreview) - Deny a Review
* [GetReviewByID](#getreviewbyid) - Retrieve a Review by ID

## ListReviews

Returns a list of all the reviews sent to the user.

Reviews are sorted by creation date, descending.


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
    res, err := s.Reviews.ListReviews(ctx)
    if err != nil {
        log.Fatal(err)
    }

    if res.Reviews != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                             | Type                                                  | Required                                              | Description                                           |
| ----------------------------------------------------- | ----------------------------------------------------- | ----------------------------------------------------- | ----------------------------------------------------- |
| `ctx`                                                 | [context.Context](https://pkg.go.dev/context#Context) | :heavy_check_mark:                                    | The context to use for the request.                   |


### Response

**[*operations.ListReviewsResponse](../../models/operations/listreviewsresponse.md), error**


## ApproveReview

Updates the specified review with an approval decision.


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
    res, err := s.Reviews.ApproveReview(ctx, operations.ApproveReviewRequest{
        ReviewUpdateParams: shared.ReviewUpdateParams{
            Reason: "perferendis",
        },
        ReviewID: "magni",
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.Review != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                          | Type                                                                               | Required                                                                           | Description                                                                        |
| ---------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| `ctx`                                                                              | [context.Context](https://pkg.go.dev/context#Context)                              | :heavy_check_mark:                                                                 | The context to use for the request.                                                |
| `request`                                                                          | [operations.ApproveReviewRequest](../../models/operations/approvereviewrequest.md) | :heavy_check_mark:                                                                 | The request object to use for the request.                                         |


### Response

**[*operations.ApproveReviewResponse](../../models/operations/approvereviewresponse.md), error**


## DenyReview

Updates the specified review with a deny decision.


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
    res, err := s.Reviews.DenyReview(ctx, operations.DenyReviewRequest{
        ReviewUpdateParams: shared.ReviewUpdateParams{
            Reason: "assumenda",
        },
        ReviewID: "ipsam",
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.Review != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                    | Type                                                                         | Required                                                                     | Description                                                                  |
| ---------------------------------------------------------------------------- | ---------------------------------------------------------------------------- | ---------------------------------------------------------------------------- | ---------------------------------------------------------------------------- |
| `ctx`                                                                        | [context.Context](https://pkg.go.dev/context#Context)                        | :heavy_check_mark:                                                           | The context to use for the request.                                          |
| `request`                                                                    | [operations.DenyReviewRequest](../../models/operations/denyreviewrequest.md) | :heavy_check_mark:                                                           | The request object to use for the request.                                   |


### Response

**[*operations.DenyReviewResponse](../../models/operations/denyreviewresponse.md), error**


## GetReviewByID

Returns the details of a review

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
    res, err := s.Reviews.GetReviewByID(ctx, operations.GetReviewByIDRequest{
        ReviewID: "alias",
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.Review != nil {
        // handle response
    }
}
```

### Parameters

| Parameter                                                                          | Type                                                                               | Required                                                                           | Description                                                                        |
| ---------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| `ctx`                                                                              | [context.Context](https://pkg.go.dev/context#Context)                              | :heavy_check_mark:                                                                 | The context to use for the request.                                                |
| `request`                                                                          | [operations.GetReviewByIDRequest](../../models/operations/getreviewbyidrequest.md) | :heavy_check_mark:                                                                 | The request object to use for the request.                                         |


### Response

**[*operations.GetReviewByIDResponse](../../models/operations/getreviewbyidresponse.md), error**

