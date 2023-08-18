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