<!-- Start SDK Example Usage [usage] -->
```go
package main

import (
	"context"
	generatedstudiosdk "github.com/speakeasy-api/speakeasy/internal/run/studio/generated-studio-sdk"
	"log"
	"os"
)

func main() {
	s := generatedstudiosdk.New(
		generatedstudiosdk.WithSecurity(os.Getenv("SECRET")),
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