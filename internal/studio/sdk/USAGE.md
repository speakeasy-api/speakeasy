<!-- Start SDK Example Usage [usage] -->
```go
package main

import (
	"context"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk/models/components"
	"log"
)

func main() {
	ctx := context.Background()

	s := sdk.New(
		sdk.WithSecurity("<YOUR_API_KEY_HERE>"),
	)

	res, err := s.GenerateOverlay(ctx, components.OverlayCompareRequestBody{
		Before: "<value>",
		After:  "<value>",
	})
	if err != nil {
		log.Fatal(err)
	}
	if res.OverlayCompareResponse != nil {
		// handle response
	}
}

```
<!-- End SDK Example Usage [usage] -->