<!-- Start SDK Example Usage [usage] -->
```go
package main

import (
	"context"
	"log"
	"openapi"
)

func main() {
	s := openapi.New(
		openapi.WithSecurity("<YOUR_API_KEY_HERE>"),
	)

	ctx := context.Background()
	res, err := s.CheckHealth(ctx)
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