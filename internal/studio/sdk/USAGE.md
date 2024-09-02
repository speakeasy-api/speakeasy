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