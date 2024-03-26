package suggestions

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/index"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/openapi-overlay/pkg/loader"
	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"
	"github.com/speakeasy-api/speakeasy/internal/schema"
	"github.com/speakeasy-api/speakeasy/internal/validation"
	openai "github.com/speakeasy-sdks/openai-go-sdk/v4"
	"github.com/speakeasy-sdks/openai-go-sdk/v4/pkg/models/shared"
)

func system(content string) shared.ChatCompletionRequestMessage {
	name := "system"
	return shared.ChatCompletionRequestMessage{
		ChatCompletionRequestSystemMessage: &shared.ChatCompletionRequestSystemMessage{
			Content: content,
			Name:    &name,
			Role:    shared.ChatCompletionRequestSystemMessageRoleSystem,
		},
	}
}

func user(content string) shared.ChatCompletionRequestMessage {
	name := "user"
	return shared.ChatCompletionRequestMessage{
		ChatCompletionRequestUserMessage: &shared.ChatCompletionRequestUserMessage{
			Content: shared.Content{
				Str: &content,
			},
			Name: &name,
			Role: shared.ChatCompletionRequestUserMessageRoleUser,
		},
	}
}

func assistant(content string) shared.ChatCompletionRequestMessage {
	name := "assistant"
	return shared.ChatCompletionRequestMessage{
		ChatCompletionRequestAssistantMessage: &shared.ChatCompletionRequestAssistantMessage{
			Content: &content,
			Name:    &name,
			Role:    shared.RoleAssistant,
		},
	}
}

func StartExampleExperiment(ctx context.Context, schemaPath string, cacheFolder string, outputFile string) error {
	_, schema, _ := schema.GetSchemaContents(ctx, schemaPath, "", "")
	err := validation.ValidateOpenAPI(ctx, schemaPath, "", "", &validation.OutputLimits{}, "", "")
	if len(os.Getenv("OPENAI_API_KEY")) == 0 {
		return errors.NewValidationError("OPENAI_API_KEY is not set", -1, nil)
	}
	if err != nil {
		return err
	}
	sdk := openai.New(openai.WithSecurity(os.Getenv("OPENAI_API_KEY")), openai.WithClient(http.DefaultClient))
	doc, err := libopenapi.NewDocumentWithConfiguration(schema, getConfig())
	if err != nil {
		return errors.NewValidationError("failed to load document", -1, err)
	}
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

func RunOnShard(ctx context.Context, sdk *openai.Gpt, shard Shard, cacheFolder string) error {
	cacheFile := filepath.Join(cacheFolder, base64(shard.Key)+".adjusted.yaml")
	if _, err := os.Stat(cacheFile); err == nil {
		return handleUpdate(ctx, cacheFolder, shard)
	}
	model := shared.TwoGpt4TurboPreview
	maxTokens := int64(4096)
	// OAS currently not declared to support text/event-stream
	shouldStream := false
	finishAt := "<Completed Task>"
	assistantMessages := []shared.ChatCompletionRequestMessage{}
	subsequentErrors := 0
	for {
		messages := []shared.ChatCompletionRequestMessage{
			system(fmt.Sprintf("Task Output a complete, modified  OpenAPI document with the following changes:\n1. For each JSON Schema which lacks an `example` field, write one. Look at the parent objects to generate them: they often have an example object defined, for the parent of each json schema. Your job is to propagate the first example into each json schema for the appropriate field.\n2. You are to do this using the `example` field. I.e. it should be in a format that is compliant to the JSON Schema type. For example, if it is `type: number`, and the parent of the field implies that it should be of value `1`, it should be set in the JSON Schema as `example: 1`. \n3. You do not need to set `example` for any composite types (object, array, oneOf, anyOf, allOf)\n4. Stay to the task: just write the modified document. You must always output yaml, no \"...\". No need for any changes except adding an example field where there isn't one.\n5. Continue from the prior assistant response, if there is one. You must continue to output a full OpenAPI document with all the same elements as in the user request. \n6. Once you are done, tell me you are done by stating \"%s\"\n", finishAt)),
			system("Example: \n```\n      requestBody:\n        content:\n          application/json:\n            schema:\n              title: UpdateContactRequest\n              type: object\n              properties:\n                first_name:\n                  type: string\n                last_name:\n                  type: string\n                display_name:\n                  type: string\n                addresses:\n                  title: UpdateContactAddresses\n                  type: array\n                  items:\n                    type: object\n                    title: UpdateContactAddress\n                    properties:\n                      address:\n                        description: The identifier that uniquely addresses an actor within a communications channel.\n                        type: string\n                      channel:\n                        $ref: '#/components/schemas/CommunicationChannel'\n                tags:\n                  $ref: '#/components/schemas/Tags'\n            examples:\n              Example 1 - Update Contact addresses:\n                value:\n                  addresses:\n                    - channel: tel\n                      address: '+37259000000'\n                    - channel: email\n                      address: ipletnjov@twilio.com\n              Example 2 - Update Contact tags:\n                value:\n                  tags:\n                    shirt_size: X-Large\n```\n => \n```\n      requestBody:\n        content:\n          application/json:\n            schema:\n              title: UpdateContactRequest\n              type: object\n              properties:\n                first_name:\n                  type: string\n                  example: Igor\n                last_name:\n                  type: string\n                  example: Pletnjov\n                display_name:\n                  type: string\n                  example: ipletnjov\n...\n```\n\n"),
			user(fmt.Sprintf("Here's my OpenAPI file: ```%s```", shard.Content)),
			user("I need to add an example field to each JSON Schema which lacks one. This should happens across the path item, as well as all component schemas (and request bodies, etc): anything that's a JSON Schema item. I should propagate the first example into each JSON Schema for the appropriate field. I should do this using the `example` field. I should stay to the task and just write the modified document, within triple back-ticks like ```\n"),
		}
		messages = append(messages, assistantMessages...)
		fmt.Printf("  Invoking ChatGPT to retrieve 4096 more tokens for operation %s.. \n", shard.Key)
		completion, err := sdk.Chat.CreateChatCompletion(ctx, shared.CreateChatCompletionRequest{
			MaxTokens: &maxTokens,
			Messages:  messages,
			Model: shared.CreateChatCompletionRequestModel{
				Two: &model,
			},
			Stop:   &shared.Stop{Str: &finishAt},
			Stream: &shouldStream,
		})
		fmt.Printf("  %s chat-gpt invoke done\n", shard.Key)
		if err != nil {
			if subsequentErrors > 5 {
				return fmt.Errorf("failed to get completion: %w", err)
			}
			subsequentErrors++
			// jitter
			jitter := time.Duration(rand.Float32() * float32(time.Minute))
			time.Sleep(jitter)
			continue
		}
		if len(completion.CreateChatCompletionResponse.Choices) != 1 {
			return fmt.Errorf("expected only 1 choice, got %d", len(completion.CreateChatCompletionResponse.Choices))
		}

		choice := completion.CreateChatCompletionResponse.Choices[0]
		content := choice.Message.Content

		assistantMessages = append(assistantMessages, assistant(*content))
		// check if we're actually done yet
		if len(*content) == 0 || strings.Contains(*content, finishAt) {
			break
		}
	}

	// merge assistantMessages back to string
	var content strings.Builder
	for _, m := range assistantMessages {
		if m.ChatCompletionRequestAssistantMessage != nil {
			content.WriteString(*m.ChatCompletionRequestAssistantMessage.Content)
		}
	}
	// trim the "Done, "```") from content
	contentWithoutDone := strings.Replace(content.String(), finishAt, "", -1)
	contentWithoutBackticks := strings.Replace(contentWithoutDone, "```", "", -1)
	// load the new result with libopenapi
	os.WriteFile(cacheFile, []byte(contentWithoutBackticks), 0o644)
	return handleUpdate(ctx, cacheFolder, shard)
}

func handleUpdate(ctx context.Context, cacheFolder string, shard Shard) error {
	overlayFilePath := filepath.Join(cacheFolder, base64(shard.Key)+".overlay.yaml")
	// if it exists, do nothing
	if _, err := os.Stat(overlayFilePath); err == nil {
		return nil
	}
	originalFile := filepath.Join(cacheFolder, base64(shard.Key)+".yaml")
	y1, err := loader.LoadSpecification(originalFile)
	if err != nil {
		return fmt.Errorf("failed to load %q: %w", originalFile, err)
	}

	adjustedFile := filepath.Join(cacheFolder, base64(shard.Key)+".adjusted.yaml")
	y2, err := loader.LoadSpecification(adjustedFile)
	if err != nil {
		return fmt.Errorf("failed to load %q: %w", adjustedFile, err)
	}
	//
	//title := fmt.Sprintf("Overlay %s => %s", schemas[0], schemas[1])
	//
	o, err := overlay.Compare("LLM", originalFile, y1, *y2)
	if err != nil {
		return fmt.Errorf("failed to compare spec files %q and %q: %w", originalFile, adjustedFile, err)
	}
	content, err := o.ToString()
	if err != nil {
		return fmt.Errorf("failed to format overlay: %w", err)
	}

	fmt.Printf("\n" + content + "\n")
	os.WriteFile(overlayFilePath, []byte(content), 0o644)
	return nil
}

func getConfig() *datamodel.DocumentConfiguration {
	return &datamodel.DocumentConfiguration{
		AllowRemoteReferences:               true,
		IgnorePolymorphicCircularReferences: true,
		IgnoreArrayCircularReferences:       true,
		ExtractRefsSequentially:             true,
	}
}

type Shard struct {
	Key     string
	Encoded string
	Content string
}

func Split(doc libopenapi.Document, cacheFolder string) ([]Shard, error) {
	v3Model, errs := doc.BuildV3Model()
	if len(errs) > 0 {
		return nil, fmt.Errorf("failed to build model: %v", errs)
	}

	paths := v3Model.Model.Paths
	if paths == nil {
		return nil, errors.NewValidationError("no paths found in document", -1, nil)
	}
	pathItems := paths.PathItems
	if pathItems == nil {
		return nil, errors.NewValidationError("no path items found in document", -1, nil)
	}

	copyDoc, err := doc.Render()
	if err != nil {
		return nil, errors.NewValidationError("failed to render document", -1, err)
	}

	shards := make([]Shard, 0)

	for pair := orderedmap.First(pathItems); pair != nil; pair = pair.Next() {
		// construct a new document with just this path item
		// encode pair.Key in base64 for the filename (as it contains "/")
		cacheFile := filepath.Join(cacheFolder, fmt.Sprintf("%s.yaml", base64(pair.Key())))
		// if already exists, use it
		if _, err := os.Stat(cacheFile); err == nil {
			cacheFileStr, err := os.ReadFile(cacheFile)
			if err != nil {
				return nil, err
			}
			shards = append(shards, Shard{Content: string(cacheFileStr), Encoded: base64(pair.Key()), Key: pair.Key()})
			continue
		}

		doc, err := libopenapi.NewDocumentWithConfiguration(copyDoc, getConfig())
		if err != nil {
			return nil, errors.NewValidationError("failed to load document", -1, err)
		}
		v3Model, _ := doc.BuildV3Model()
		v3Model.Model.Paths.PathItems = orderedmap.New[string, *v3.PathItem]()
		v3Model.Model.Paths.PathItems.Set(pair.Key(), pair.Value())
		// eliminate all the now-orphaned schemas
		doc, v3Model, err = removeOrphans(doc, v3Model)
		if err != nil {
			return nil, err
		}
		// render the document to our shard
		shard, err := v3Model.Model.Render()
		if err != nil {
			return nil, errors.NewValidationError("failed to render document", -1, err)
		}
		// cache it
		err = os.WriteFile(cacheFile, shard, 0o644)
		shards = append(shards, Shard{Content: string(shard), Encoded: base64(pair.Key()), Key: pair.Key()})
	}
	return shards, nil
}

func base64(key string) string {
	return b64.StdEncoding.EncodeToString([]byte(key))
}

func removeOrphans(doc libopenapi.Document, model *libopenapi.DocumentModel[v3.Document]) (libopenapi.Document, *libopenapi.DocumentModel[v3.Document], error) {
	_, doc, model, errs := doc.RenderAndReload()
	// remove nil errs
	var nonNilErrs []error
	for _, e := range errs {
		if e != nil {
			nonNilErrs = append(nonNilErrs, e)
		}
	}
	if len(nonNilErrs) > 0 {
		return nil, nil, fmt.Errorf("failed to render and reload document: %v", errs)
	}

	components := model.Model.Components
	context := model
	allRefs := context.Index.GetAllReferences()
	schemasIdx := context.Index.GetAllComponentSchemas()
	responsesIdx := context.Index.GetAllResponses()
	parametersIdx := context.Index.GetAllParameters()
	examplesIdx := context.Index.GetAllExamples()
	requestBodiesIdx := context.Index.GetAllRequestBodies()
	headersIdx := context.Index.GetAllHeaders()
	securitySchemesIdx := context.Index.GetAllSecuritySchemes()
	linksIdx := context.Index.GetAllLinks()
	callbacksIdx := context.Index.GetAllCallbacks()
	mappedRefs := context.Index.GetMappedReferences()

	checkOpenAPISecurity := func(key string) bool {
		if strings.Contains(key, "securitySchemes") {
			segs := strings.Split(key, "/")
			def := segs[len(segs)-1]
			for r := range context.Index.GetSecurityRequirementReferences() {
				if r == def {
					return true
				}
			}
		}
		return false
	}

	// create poly maps.
	oneOfRefs := make(map[string]*index.Reference)
	allOfRefs := make(map[string]*index.Reference)
	anyOfRefs := make(map[string]*index.Reference)

	// include all polymorphic references.
	for _, ref := range context.Index.GetPolyAllOfReferences() {
		allOfRefs[ref.Definition] = ref
	}
	for _, ref := range context.Index.GetPolyOneOfReferences() {
		oneOfRefs[ref.Definition] = ref
	}
	for _, ref := range context.Index.GetPolyAnyOfReferences() {
		anyOfRefs[ref.Definition] = ref
	}

	notUsed := make(map[string]*index.Reference)
	mapsToSearch := []map[string]*index.Reference{
		schemasIdx,
		responsesIdx,
		parametersIdx,
		examplesIdx,
		requestBodiesIdx,
		headersIdx,
		securitySchemesIdx,
		linksIdx,
		callbacksIdx,
	}

	for _, resultMap := range mapsToSearch {
		for key, ref := range resultMap {

			u := strings.Split(key, "#/")
			keyAlt := key
			if len(u) == 2 {
				if u[0] == "" {
					keyAlt = fmt.Sprintf("%s#/%s", context.Index.GetSpecAbsolutePath(), u[1])
				}
			}

			if allRefs[key] == nil && allRefs[keyAlt] == nil {
				found := false

				if oneOfRefs[key] != nil || allOfRefs[key] != nil || anyOfRefs[key] != nil {
					found = true
				}

				if mappedRefs[key] != nil || mappedRefs[keyAlt] != nil {
					found = true
				}

				if !found {
					found = checkOpenAPISecurity(key)
				}

				if !found {
					notUsed[key] = ref
				}
			}
		}
	}

	// let's start killing orphans
	anyRemoved := false
	schemas := components.Schemas
	toDelete := make([]string, 0)
	for pair := orderedmap.First(schemas); pair != nil; pair = pair.Next() {
		// remove all schemas that are not referenced
		if !isReferenced(pair.Key(), "schemas", notUsed) {
			toDelete = append(toDelete, pair.Key())
			fmt.Printf("dropped #/components/schemas/%s\n", pair.Key())
			anyRemoved = true
		}
	}
	for _, key := range toDelete {
		schemas.Delete(key)
	}

	responses := components.Responses
	toDelete = make([]string, 0)
	for pair := orderedmap.First(responses); pair != nil; pair = pair.Next() {
		// remove all responses that are not referenced
		if !isReferenced(pair.Key(), "responses", notUsed) {
			toDelete = append(toDelete, pair.Key())
			fmt.Printf("dropped #/components/responses/%s\n", pair.Key())
			anyRemoved = true
		}
	}
	for _, key := range toDelete {
		responses.Delete(key)
	}
	parameters := components.Parameters
	toDelete = make([]string, 0)
	for pair := orderedmap.First(parameters); pair != nil; pair = pair.Next() {
		// remove all parameters that are not referenced
		if !isReferenced(pair.Key(), "parameters", notUsed) {
			toDelete = append(toDelete, pair.Key())
			anyRemoved = true
		}
	}
	for _, key := range toDelete {
		parameters.Delete(key)
	}
	examples := components.Examples
	toDelete = make([]string, 0)
	for pair := orderedmap.First(examples); pair != nil; pair = pair.Next() {
		// remove all examples that are not referenced
		if !isReferenced(pair.Key(), "examples", notUsed) {
			toDelete = append(toDelete, pair.Key())
			anyRemoved = true
		}
	}
	for _, key := range toDelete {
		examples.Delete(key)
	}
	requestBodies := components.RequestBodies
	toDelete = make([]string, 0)
	for pair := orderedmap.First(requestBodies); pair != nil; pair = pair.Next() {
		// remove all requestBodies that are not referenced
		if !isReferenced(pair.Key(), "requestBodies", notUsed) {
			toDelete = append(toDelete, pair.Key())
			anyRemoved = true
		}
	}
	for _, key := range toDelete {
		requestBodies.Delete(key)
	}

	headers := components.Headers
	toDelete = make([]string, 0)
	for pair := orderedmap.First(headers); pair != nil; pair = pair.Next() {
		// remove all headers that are not referenced
		if !isReferenced(pair.Key(), "headers", notUsed) {
			toDelete = append(toDelete, pair.Key())
			anyRemoved = true
		}
	}
	for _, key := range toDelete {
		headers.Delete(key)
	}
	if anyRemoved {
		return removeOrphans(doc, model)
	}
	return doc, model, nil
}

func isReferenced(key string, within string, notUsed map[string]*index.Reference) bool {
	ref := fmt.Sprintf("#/components/%s/%s", within, key)
	if notUsed[ref] != nil {
		return false
	}
	return true
}
