<!-- Start SDK Example Usage [usage] -->
```go
package main

import (
	"context"
	"log"
	"openapi"
	"os"
)

func main() {
	s := openapi.New(
		openapi.WithSecurity(os.Getenv("SECRET")),
	)

	ctx := context.Background()
	res, err := s.CheckHealth(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if res.HealthResponse != nil {
		// handle response
	}
}

```
<!-- End SDK Example Usage [usage] -->