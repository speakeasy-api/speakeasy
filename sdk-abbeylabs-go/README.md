# openapi

<!-- Start SDK Installation -->
## SDK Installation

```bash
go get openapi
```
<!-- End SDK Installation -->

## SDK Example Usage
<!-- Start SDK Example Usage -->


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
    res, err := s.APIKeys.CreateAPIKey(ctx, shared.APIKeysCreateParams{
        Name: "Terrence Rau",
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.APIKey != nil {
        // handle response
    }
}
```
<!-- End SDK Example Usage -->

<!-- Start SDK Available Operations -->
## Available Resources and Operations


### [APIKeys](docs/sdks/apikeys/README.md)

* [CreateAPIKey](docs/sdks/apikeys/README.md#createapikey) - Create an API Key
* [GetAPIKeys](docs/sdks/apikeys/README.md#getapikeys) - List API Keys
* [DeleteAPIKey](docs/sdks/apikeys/README.md#deleteapikey) - Delete an API Key

### [ConnectionSpecs](docs/sdks/connectionspecs/README.md)

* [ListConnectionSpecs](docs/sdks/connectionspecs/README.md#listconnectionspecs) - List Connection Specs

### [Connections](docs/sdks/connections/README.md)

* [CreateConnection](docs/sdks/connections/README.md#createconnection) - Create a Connection
* [GetConnection](docs/sdks/connections/README.md#getconnection) - Retrieve a Connection by ID
* [ListConnections](docs/sdks/connections/README.md#listconnections) - List Connections
* [UpdateConnection](docs/sdks/connections/README.md#updateconnection) - Update a Connection

### [GrantKits](docs/sdks/grantkits/README.md)

* [CreateGrantKit](docs/sdks/grantkits/README.md#creategrantkit) - Create a Grant Kit
* [GetGrantKits](docs/sdks/grantkits/README.md#getgrantkits) - List Grant Kits
* [DeleteGrantKit](docs/sdks/grantkits/README.md#deletegrantkit) - Delete a Grant Kit
* [GetGrantKitByID](docs/sdks/grantkits/README.md#getgrantkitbyid) - Retrieve a Grant Kit by ID
* [ListGrantKitVersionsByID](docs/sdks/grantkits/README.md#listgrantkitversionsbyid) - List Grant Kit Versions of a Grant Kit ID
* [UpdateGrantKit](docs/sdks/grantkits/README.md#updategrantkit) - Update a Grant Kit

### [Grants](docs/sdks/grants/README.md)

* [ListGrants](docs/sdks/grants/README.md#listgrants) - List Grants
* [GetGrantByID](docs/sdks/grants/README.md#getgrantbyid) - Retrieve a Grant by ID
* [RevokeGrant](docs/sdks/grants/README.md#revokegrant) - Revoke a Grant by ID

### [Identities](docs/sdks/identities/README.md)

* [CreateIdentity](docs/sdks/identities/README.md#createidentity) - Create an Identity
* [DeleteIdentity](docs/sdks/identities/README.md#deleteidentity) - Delete an Identity
* [GetIdentity](docs/sdks/identities/README.md#getidentity) - Retrieve an Identity

### [Requests](docs/sdks/requests/README.md)

* [CancelRequestByID](docs/sdks/requests/README.md#cancelrequestbyid) - Cancel a Request by ID
* [CreateRequest](docs/sdks/requests/README.md#createrequest) - Create a Request
* [GetRequestByID](docs/sdks/requests/README.md#getrequestbyid) - Retrieve a Request by ID
* [ListRequests](docs/sdks/requests/README.md#listrequests) - List Requests

### [Reviews](docs/sdks/reviews/README.md)

* [ListReviews](docs/sdks/reviews/README.md#listreviews) - List Reviews
* [ApproveReview](docs/sdks/reviews/README.md#approvereview) - Approve a Review
* [DenyReview](docs/sdks/reviews/README.md#denyreview) - Deny a Review
* [GetReviewByID](docs/sdks/reviews/README.md#getreviewbyid) - Retrieve a Review by ID

### [Tasks](docs/sdks/tasks/README.md)

* [CreateTask](docs/sdks/tasks/README.md#createtask) - Creates a new task.
* [GetTaskByID](docs/sdks/tasks/README.md#gettaskbyid) - Returns the details of a task.
* [ListTasks](docs/sdks/tasks/README.md#listtasks) - Returns a list of tasks.
Tasks are sorted by creation date, descending.

* [UpdateTask](docs/sdks/tasks/README.md#updatetask) - Updates a task's attributes.
This performs a full update that replaces the entire set of attributes.

<!-- End SDK Available Operations -->

### Maturity

This SDK is in beta, and there may be breaking changes between versions without a major version update. Therefore, we recommend pinning usage
to a specific package version. This way, you can install the same version each time without breaking changes unless you are intentionally
looking for the latest version.

### Contributions

While we value open-source contributions to this SDK, this library is generated programmatically.
Feel free to open a PR or a Github issue as a proof of concept and we'll do our best to include it in a future release!

### SDK Created by [Speakeasy](https://docs.speakeasyapi.dev/docs/using-speakeasy/client-sdks)
