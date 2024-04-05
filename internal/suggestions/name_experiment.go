package suggestions

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pb33f/libopenapi"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"
	"github.com/speakeasy-api/speakeasy/internal/schema"
	"github.com/speakeasy-api/speakeasy/internal/validation"
	openai "github.com/speakeasy-sdks/openai-go-sdk/v4"
)

func StartNameExperiment(ctx context.Context, schemaPath string, cacheFolder string, outputFile string) error {
	_, schema, _ := schema.GetSchemaContents(ctx, schemaPath, "", "")

	err := validation.ValidateOpenAPI(ctx, schemaPath, "", "", &validation.OutputLimits{}, "", "")
	if err != nil {
		return err
	}

	// Initialize the OpenAI SDK
	sdk := openai.New(openai.WithSecurity(os.Getenv("OPENAI_API_KEY")), openai.WithClient(http.DefaultClient))

	// Load the document
	doc, err := libopenapi.NewDocumentWithConfiguration(schema, getConfig())

	if err != nil {
		return errors.NewValidationError("failed to load document", -1, err)
	}

	// Split the document into shards
	splitSchema, err := Split(doc, cacheFolder)
	if err != nil {
		return err
	}

	parallelism := 10

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, parallelism)
	i := 0

	for _, shard := range splitSchema {

		wg.Add(1)

		semaphore <- struct{}{} // Acquire a token
		i++

		go func(shard Shard, i int) {
			<-semaphore // Release the token
			defer wg.Done()
			fmt.Printf("Processing shard %s (%v / %v)\n", shard.Key, i, len(splitSchema))
			jitter := time.Duration(rand.Float32() * float32(time.Second) * 15)
			time.Sleep(jitter)
			errMessage := RunOnShard(ctx, sdk, shard, cacheFolder)
			if errMessage != nil {
				fmt.Println(errMessage)
			}
			fmt.Printf("Shard %s complete (%v / %v)\n", shard.Key, i, len(splitSchema))
			<-semaphore // Release the token
		}(shard, i)
	}

	wg.Wait() // Wait for all goroutines to finish

	schemaOut, err := filepath.Abs(schemaPath)
	if err != nil {
		return err
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	relPath, err := filepath.Rel(wd, schemaOut)
	if err != nil {
		return err
	}

	combinedOverlay := overlay.Overlay{
		Version: "0.0.1",
		Info: overlay.Info{
			Title:   "Overlay File created by Speakeasy Suggest",
			Version: "0.0.1",
		},
		Extends: "file://" + relPath,
		Actions: nil,
	}

	usedKeys := make(map[string]bool)
	for _, shard := range splitSchema {
		// merge the overlays together against the original document
		overlayFile := filepath.Join(cacheFolder, base64(shard.Key)+".overlay.yaml")
		newDoc, err := overlay.Parse(overlayFile)
		if err != nil {
			continue
		}
		for _, action := range newDoc.Actions {
			if action.Remove {
				continue
			}
			// add if not seen before
			if _, ok := usedKeys[action.Target]; !ok {
				usedKeys[action.Target] = true
				combinedOverlay.Actions = append(combinedOverlay.Actions, action)
			}
		}
	}

	// write the new document to the output file
	combined, err := combinedOverlay.ToString()
	if err != nil {
		return err
	}

	return os.WriteFile(outputFile, []byte(combined), 0o644)

}
