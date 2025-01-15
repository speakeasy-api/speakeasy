<!-- Start SDK Example Usage [usage] -->
```go
package main

import (
	"context"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk/models/operations"
	"log"
)

func main() {
	ctx := context.Background()

	s := sdk.New(
		sdk.WithSecurity("<YOUR_API_KEY_HERE>"),
	)

	res, err := s.GenerateOverlay(ctx, operations.GenerateOverlayRequestBody{
		Before: "<value>",
		After:  "<value>",
	})
	if err != nil {
		log.Fatal(err)
	}
	if res.Object != nil {
		// handle response
	}
}

```
<!-- End SDK Example Usage [usage] -->