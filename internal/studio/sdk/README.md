# github.com/speakeasy-api/speakeasy/internal/run/studio/generated-studio-sdk

<div align="left">
    <a href="https://speakeasyapi.dev/"><img src="https://custom-icon-badges.demolab.com/badge/-Built%20By%20Speakeasy-212015?style=for-the-badge&logoColor=FBE331&logo=speakeasy&labelColor=545454" /></a>
    <a href="https://opensource.org/licenses/MIT">
        <img src="https://img.shields.io/badge/License-MIT-blue.svg" style="width: 100px; height: 28px;" />
    </a>
</div>


## 🏗 **Welcome to your new SDK!** 🏗

It has been generated successfully based on your OpenAPI spec. However, it is not yet ready for production use. Here are some next steps:
- [ ] 🛠 Make your SDK feel handcrafted by [customizing it](https://www.speakeasyapi.dev/docs/customize-sdks)
- [ ] ♻️ Refine your SDK quickly by iterating locally with the [Speakeasy CLI](https://github.com/speakeasy-api/speakeasy)
- [ ] 🎁 Publish your SDK to package managers by [configuring automatic publishing](https://www.speakeasyapi.dev/docs/advanced-setup/publish-sdks)
- [ ] ✨ When ready to productionize, delete this section from the README

<!-- Start Summary [summary] -->
## Summary


<!-- End Summary [summary] -->

<!-- Start Table of Contents [toc] -->
## Table of Contents

* [SDK Installation](#sdk-installation)
* [SDK Example Usage](#sdk-example-usage)
* [Available Resources and Operations](#available-resources-and-operations)
* [Server-sent event streaming](#server-sent-event-streaming)
* [Retries](#retries)
* [Error Handling](#error-handling)
* [Server Selection](#server-selection)
* [Custom HTTP Client](#custom-http-client)
* [Authentication](#authentication)
* [Special Types](#special-types)
<!-- End Table of Contents [toc] -->

<!-- Start SDK Installation [installation] -->
## SDK Installation

To add the SDK as a dependency to your project:
```bash
go get github.com/speakeasy-api/speakeasy/internal/studio/sdk
```
<!-- End SDK Installation [installation] -->

<!-- Start SDK Example Usage [usage] -->
## SDK Example Usage

### Example

```go
package main

import (
	"context"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk"
	"log"
)

func main() {
	s := sdk.New(
		sdk.WithSecurity("<YOUR_API_KEY_HERE>"),
	)

	ctx := context.Background()
	res, err := s.Health.Check(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if res.HealthResponse != nil {
		defer res.HealthResponse.Close()

		for res.HealthResponse.Next() {
			event := res.HealthResponse.Value()
			log.Print(event)
			// Handle the event
		}
	}
}

```
<!-- End SDK Example Usage [usage] -->

<!-- Start Available Resources and Operations [operations] -->
## Available Resources and Operations

### [Health](docs/sdks/health/README.md)

* [Check](docs/sdks/health/README.md#check) - Health Check

### [Runs](docs/sdks/runs/README.md)

* [GetLast](docs/sdks/runs/README.md#getlast) - Run

### [Run](docs/sdks/run/README.md)

* [Execute](docs/sdks/run/README.md#execute) - Run

### [Suggest](docs/sdks/suggest/README.md)

* [MethodNames](docs/sdks/suggest/README.md#methodnames) - Suggest Method Names
<!-- End Available Resources and Operations [operations] -->

<!-- Start Retries [retries] -->
## Retries

Some of the endpoints in this SDK support retries. If you use the SDK without any configuration, it will fall back to the default retry strategy provided by the API. However, the default retry strategy can be overridden on a per-operation basis, or across the entire SDK.

To change the default retry strategy for a single API call, simply provide a `retry.Config` object to the call by using the `WithRetries` option:
```go
package main

import (
	"context"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk/retry"
	"log"
	"models/operations"
)

func main() {
	s := sdk.New(
		sdk.WithSecurity("<YOUR_API_KEY_HERE>"),
	)

	ctx := context.Background()
	res, err := s.Health.Check(ctx, operations.WithRetries(
		retry.Config{
			Strategy: "backoff",
			Backoff: &retry.BackoffStrategy{
				InitialInterval: 1,
				MaxInterval:     50,
				Exponent:        1.1,
				MaxElapsedTime:  100,
			},
			RetryConnectionErrors: false,
		}))
	if err != nil {
		log.Fatal(err)
	}
	if res.HealthResponse != nil {
		defer res.HealthResponse.Close()

		for res.HealthResponse.Next() {
			event := res.HealthResponse.Value()
			log.Print(event)
			// Handle the event
		}
	}
}

```

If you'd like to override the default retry strategy for all operations that support retries, you can use the `WithRetryConfig` option at SDK initialization:
```go
package main

import (
	"context"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk/retry"
	"log"
)

func main() {
	s := sdk.New(
		sdk.WithRetryConfig(
			retry.Config{
				Strategy: "backoff",
				Backoff: &retry.BackoffStrategy{
					InitialInterval: 1,
					MaxInterval:     50,
					Exponent:        1.1,
					MaxElapsedTime:  100,
				},
				RetryConnectionErrors: false,
			}),
		sdk.WithSecurity("<YOUR_API_KEY_HERE>"),
	)

	ctx := context.Background()
	res, err := s.Health.Check(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if res.HealthResponse != nil {
		defer res.HealthResponse.Close()

		for res.HealthResponse.Next() {
			event := res.HealthResponse.Value()
			log.Print(event)
			// Handle the event
		}
	}
}

```
<!-- End Retries [retries] -->

<!-- Start Error Handling [errors] -->
## Error Handling

Handling errors in this SDK should largely match your expectations.  All operations return a response object or an error, they will never return both.  When specified by the OpenAPI spec document, the SDK will return the appropriate subclass.

| Error Object       | Status Code        | Content Type       |
| ------------------ | ------------------ | ------------------ |
| sdkerrors.SDKError | 4xx-5xx            | */*                |

### Example

```go
package main

import (
	"context"
	"errors"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk/models/sdkerrors"
	"log"
)

func main() {
	s := sdk.New(
		sdk.WithSecurity("<YOUR_API_KEY_HERE>"),
	)

	ctx := context.Background()
	res, err := s.Health.Check(ctx)
	if err != nil {

		var e *sdkerrors.SDKError
		if errors.As(err, &e) {
			// handle error
			log.Fatal(e.Error())
		}
	}
}

```
<!-- End Error Handling [errors] -->

<!-- Start Server Selection [server] -->
## Server Selection

### Select Server by Index

You can override the default server globally using the `WithServerIndex` option when initializing the SDK client instance. The selected server will then be used as the default on the operations that use it. This table lists the indexes associated with the available servers:

| # | Server | Variables |
| - | ------ | --------- |
| 0 | `http://localhost:{port}` | `port` (default is `8080`) |

#### Example

```go
package main

import (
	"context"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk"
	"log"
)

func main() {
	s := sdk.New(
		sdk.WithServerIndex(0),
		sdk.WithSecurity("<YOUR_API_KEY_HERE>"),
	)

	ctx := context.Background()
	res, err := s.Health.Check(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if res.HealthResponse != nil {
		defer res.HealthResponse.Close()

		for res.HealthResponse.Next() {
			event := res.HealthResponse.Value()
			log.Print(event)
			// Handle the event
		}
	}
}

```

#### Variables

Some of the server options above contain variables. If you want to set the values of those variables, the following options are provided for doing so:
 * `WithPort string`

### Override Server URL Per-Client

The default server can also be overridden globally using the `WithServerURL` option when initializing the SDK client instance. For example:
```go
package main

import (
	"context"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk"
	"log"
)

func main() {
	s := sdk.New(
		sdk.WithServerURL("http://localhost:{port}"),
		sdk.WithSecurity("<YOUR_API_KEY_HERE>"),
	)

	ctx := context.Background()
	res, err := s.Health.Check(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if res.HealthResponse != nil {
		defer res.HealthResponse.Close()

		for res.HealthResponse.Next() {
			event := res.HealthResponse.Value()
			log.Print(event)
			// Handle the event
		}
	}
}

```
<!-- End Server Selection [server] -->

<!-- Start Custom HTTP Client [http-client] -->
## Custom HTTP Client

The Go SDK makes API calls that wrap an internal HTTP client. The requirements for the HTTP client are very simple. It must match this interface:

```go
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}
```

The built-in `net/http` client satisfies this interface and a default client based on the built-in is provided by default. To replace this default with a client of your own, you can implement this interface yourself or provide your own client configured as desired. Here's a simple example, which adds a client with a 30 second timeout.

```go
import (
	"net/http"
	"time"
	"github.com/myorg/your-go-sdk"
)

var (
	httpClient = &http.Client{Timeout: 30 * time.Second}
	sdkClient  = sdk.New(sdk.WithClient(httpClient))
)
```

This can be a convenient way to configure timeouts, cookies, proxies, custom headers, and other low-level configuration.
<!-- End Custom HTTP Client [http-client] -->

<!-- Start Special Types [types] -->
## Special Types


<!-- End Special Types [types] -->

<!-- Start Authentication [security] -->
## Authentication

### Per-Client Security Schemes

This SDK supports the following security scheme globally:

| Name     | Type     | Scheme   |
| -------- | -------- | -------- |
| `Secret` | apiKey   | API key  |

You can configure it using the `WithSecurity` option when initializing the SDK client instance. For example:
```go
package main

import (
	"context"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk"
	"log"
)

func main() {
	s := sdk.New(
		sdk.WithSecurity("<YOUR_API_KEY_HERE>"),
	)

	ctx := context.Background()
	res, err := s.Health.Check(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if res.HealthResponse != nil {
		defer res.HealthResponse.Close()

		for res.HealthResponse.Next() {
			event := res.HealthResponse.Value()
			log.Print(event)
			// Handle the event
		}
	}
}

```
<!-- End Authentication [security] -->

<!-- Start Server-sent event streaming [eventstream] -->
## Server-sent event streaming

[Server-sent events][mdn-sse] are used to stream content from certain
operations. These operations will expose the stream as an iterable that
can be consumed using a simple `for` loop. The loop will
terminate when the server no longer has any events to send and closes the
underlying connection.

```go
package main

import (
	"context"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk"
	"log"
)

func main() {
	s := sdk.New(
		sdk.WithSecurity("<YOUR_API_KEY_HERE>"),
	)

	ctx := context.Background()
	res, err := s.Health.Check(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if res.HealthResponse != nil {
		defer res.HealthResponse.Close()

		for res.HealthResponse.Next() {
			event := res.HealthResponse.Value()
			log.Print(event)
			// Handle the event
		}
	}
}

```

[mdn-sse]: https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events
<!-- End Server-sent event streaming [eventstream] -->

<!-- Placeholder for Future Speakeasy SDK Sections -->

# Development

## Maturity

This SDK is in beta, and there may be breaking changes between versions without a major version update. Therefore, we recommend pinning usage
to a specific package version. This way, you can install the same version each time without breaking changes unless you are intentionally
looking for the latest version.

## Contributions

While we value open-source contributions to this SDK, this library is generated programmatically. Any manual changes added to internal files will be overwritten on the next generation. 
We look forward to hearing your feedback. Feel free to open a PR or an issue with a proof of concept and we'll do our best to include it in a future release. 

### SDK Created by [Speakeasy](https://docs.speakeasyapi.dev/docs/using-speakeasy/client-sdks)
