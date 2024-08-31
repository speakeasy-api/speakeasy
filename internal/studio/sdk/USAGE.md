<!-- Start SDK Example Usage [usage] -->
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
	res, err := s.GetRun(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if res.RunResponse != nil {
		// handle response
	}
}

```
<!-- End SDK Example Usage [usage] -->