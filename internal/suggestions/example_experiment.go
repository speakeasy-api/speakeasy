package suggestions

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/index"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/speakeasy/internal/schema"
	"github.com/speakeasy-api/speakeasy/internal/validation"
	"github.com/speakeasy-api/speakeasy/pkg/merge"
	openai "github.com/speakeasy-sdks/openai-go-sdk/v4"
	"github.com/speakeasy-sdks/openai-go-sdk/v4/pkg/models/shared"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	err := validation.ValidateOpenAPI(ctx, schemaPath, "", "", &validation.OutputLimits{})
	if err != nil {
		return err
	}
	sdk := openai.New(openai.WithSecurity(os.Getenv("OPENAI_API_KEY")), openai.WithClient(http.DefaultClient))
	doc, err := libopenapi.NewDocumentWithConfiguration(schema, getConfig())
	if err != nil {
		return errors.NewValidationError("failed to load document", -1, err)
	}
	v3OriginalDoc, errs := doc.BuildV3Model()
	if len(errs) > 0 {
		return errors.NewValidationError("failed to build model", -1, err)
	}
	splitSchema, err := Split(doc, cacheFolder)
	if err != nil {
		return err
	}

	for _, shard := range splitSchema {
		cacheFile := filepath.Join(cacheFolder, base64(shard.Key)+".adjusted.yaml")
		if _, err := os.Stat(cacheFile); err == nil {
			content, err := os.ReadFile(cacheFile)
			if err != nil {
				return err
			}
			updatedDoc, err := libopenapi.NewDocumentWithConfiguration([]byte(content), getConfig())
			if err != nil {
				return err
			}
			v3UpdatedDoc, errs := updatedDoc.BuildV3Model()
			if len(errs) > 0 {
				return errors.NewValidationError("failed to build model", -1, err)
			}

			_, errs = merge.MergeDocuments(&v3OriginalDoc.Model, &v3UpdatedDoc.Model)
			if len(errs) > 0 {
				return fmt.Errorf("failed to merge documents: %v", errs)
			}
		}
		model := shared.TwoGpt4TurboPreview
		maxTokens := int64(4096)
		// OAS currently not declared to support text/event-stream
		shouldStream := false
		finishAt := "(done)"
		assistantMessages := []shared.ChatCompletionRequestMessage{}
		subsequentErrors := 0
		for {
			messages := []shared.ChatCompletionRequestMessage{
				system(fmt.Sprintf("Task Output a complete, modified  OpenAPI document with the following changes:\n1. For each JSON Schema which lacks an `example` field, write one. Look at the parent objects to generate them: they often have an example object defined, for the parent of each json schema. Your job is to propagate the first example into each json schema for the appropriate field.\n2. You are to do this using the `example` field. I.e. it should be in a format that is compliant to the JSON Schema type. For example, if it is `type: number`, and the parent of the field implies that it should be of value `1`, it should be set in the JSON Schema as `example: 1`. \n3. You do not need to set `example` for any composite types (object, array, oneOf, anyOf, allOf)\n4. Stay to the task: just write the modified document. No need for any changes except adding an example field where there isn't one.\n5. Continue from the prior assistant response, if there is one. You must continue to output a full OpenAPI document with all the same elements as in the user request. \n", finishAt)),
				system("Example: \n```\n      requestBody:\n        content:\n          application/json:\n            schema:\n              title: UpdateContactRequest\n              type: object\n              properties:\n                first_name:\n                  type: string\n                last_name:\n                  type: string\n                display_name:\n                  type: string\n                addresses:\n                  title: UpdateContactAddresses\n                  type: array\n                  items:\n                    type: object\n                    title: UpdateContactAddress\n                    properties:\n                      address:\n                        description: The identifier that uniquely addresses an actor within a communications channel.\n                        type: string\n                      channel:\n                        $ref: '#/components/schemas/CommunicationChannel'\n                tags:\n                  $ref: '#/components/schemas/Tags'\n            examples:\n              Example 1 - Update Contact addresses:\n                value:\n                  addresses:\n                    - channel: tel\n                      address: '+37259000000'\n                    - channel: email\n                      address: ipletnjov@twilio.com\n              Example 2 - Update Contact tags:\n                value:\n                  tags:\n                    shirt_size: X-Large\n```\n => \n```\n      requestBody:\n        content:\n          application/json:\n            schema:\n              title: UpdateContactRequest\n              type: object\n              properties:\n                first_name:\n                  type: string\n                  example: Igor\n                last_name:\n                  type: string\n                  example: Pletnjov\n                display_name:\n                  type: string\n                  example: ipletnjov\n...\n```\n\n"),
				user(fmt.Sprintf("Here's my OpenAPI file: ```%s```", shard.Content)),
				user("I need to add an example field to each JSON Schema which lacks one. This should happens across the path item, as well as all component schemas (and request bodies, etc): anything that's a JSON Schema item. I should propagate the first example into each JSON Schema for the appropriate field. I should do this using the `example` field. I should stay to the task and just write the modified document, within triple back-ticks like ```\n"),
			}
			messages = append(messages, assistantMessages...)
			fmt.Printf("Invoking ChatGPT to retrieve 4096 more tokens for operation %s.. ", shard.Key)
			completion, err := sdk.Chat.CreateChatCompletion(ctx, shared.CreateChatCompletionRequest{
				MaxTokens: &maxTokens,
				Messages:  messages,
				Model: shared.CreateChatCompletionRequestModel{
					Two: &model,
				},
				Stop:   &shared.Stop{Str: &finishAt},
				Stream: &shouldStream,
			})
			fmt.Printf("Done\n")
			if err != nil {
				if subsequentErrors > 5 {
					break
				}
				subsequentErrors++
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
		fmt.Printf("Content: %s\n", contentWithoutBackticks)
		os.WriteFile(cacheFile, []byte(contentWithoutBackticks), 0644)
		updatedDoc, err := libopenapi.NewDocumentWithConfiguration([]byte(contentWithoutBackticks), getConfig())
		if err != nil {
			return err
		}
		v3UpdatedDoc, errs := updatedDoc.BuildV3Model()
		if len(errs) > 0 {
			return errors.NewValidationError("failed to build model", -1, err)
		}

		_, errs = merge.MergeDocuments(&v3OriginalDoc.Model, &v3UpdatedDoc.Model)
		if len(errs) > 0 {
			return fmt.Errorf("failed to merge documents: %v", errs)
		}
	}
	newDoc, err := doc.Render()
	if err != nil {
		return errors.NewValidationError("failed to render document", -1, err)
	}
	fmt.Printf("Content: %s\n", newDoc)
	return os.WriteFile(outputFile, newDoc, 0644)
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
			shards = append(shards, Shard{Content: string(cacheFileStr), Key: pair.Key()})
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
		err = os.WriteFile(cacheFile, shard, 0644)
		shards = append(shards, Shard{Content: string(shard), Key: pair.Key()})
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
			var keyAlt = key
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
	for pair := orderedmap.First(schemas); pair != nil; pair = pair.Next() {
		// remove all schemas that are not referenced
		if !isReferenced(pair.Key(), "schemas", notUsed) {
			schemas.Delete(pair.Key())
			fmt.Printf("dropped #/components/schemas/%s\n", pair.Key())
			anyRemoved = true
		}
	}
	responses := components.Responses
	for pair := orderedmap.First(responses); pair != nil; pair = pair.Next() {
		// remove all responses that are not referenced
		if !isReferenced(pair.Key(), "responses", notUsed) {
			responses.Delete(pair.Key())
			fmt.Printf("dropped #/components/responses/%s\n", pair.Key())
			anyRemoved = true
		}
	}
	parameters := components.Parameters
	for pair := orderedmap.First(parameters); pair != nil; pair = pair.Next() {
		// remove all parameters that are not referenced
		if !isReferenced(pair.Key(), "parameters", notUsed) {
			parameters.Delete(pair.Key())
			anyRemoved = true
		}
	}
	examples := components.Examples
	for pair := orderedmap.First(examples); pair != nil; pair = pair.Next() {
		// remove all examples that are not referenced
		if !isReferenced(pair.Key(), "examples", notUsed) {
			examples.Delete(pair.Key())
			anyRemoved = true
		}
	}
	requestBodies := components.RequestBodies
	for pair := orderedmap.First(requestBodies); pair != nil; pair = pair.Next() {
		// remove all requestBodies that are not referenced
		if !isReferenced(pair.Key(), "requestBodies", notUsed) {
			requestBodies.Delete(pair.Key())
			anyRemoved = true
		}
	}
	headers := components.Headers
	for pair := orderedmap.First(headers); pair != nil; pair = pair.Next() {
		// remove all headers that are not referenced
		if !isReferenced(pair.Key(), "headers", notUsed) {
			headers.Delete(pair.Key())
			anyRemoved = true
		}
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
