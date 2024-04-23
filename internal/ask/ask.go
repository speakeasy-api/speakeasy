package ask

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/inkeep/ai-api-go/models/sdkerrors"
	"github.com/speakeasy-api/huh"
	charm_internal "github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/log"
)

var (
	boldRegex   = regexp.MustCompile(`\*\*(.*?)\*\*`)
	italicRegex = regexp.MustCompile(`\*(.*?)\*`)
	linkRegex   = regexp.MustCompile(`\[\(?(.*?)\)?\]\((https?:\/\/[^\s]+)\)`)
)

type RequestPayload struct {
	SessionID string `json:"session_id,omitempty"`
	Message   string `json:"Message"`
}

type ChatResponse struct {
	SessionID string `json:"ChatSessionID"`
	Message   string `json:"Message"`
}

const ApiURL = "https://api.prod.speakeasyapi.dev"

var baseURL = ApiURL

func makeHTTPRequest(ctx context.Context, url string, payload RequestPayload) (ChatResponse, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return ChatResponse{}, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return ChatResponse{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return ChatResponse{}, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ChatResponse{}, err
	}

	var chatResponse ChatResponse
	err = json.Unmarshal(body, &chatResponse)
	if err != nil {
		return ChatResponse{}, err
	}

	return chatResponse, nil
}

func Ask(ctx context.Context, message string, sessionID string) (string, error) {
	logger := log.From(ctx)
	var endpoint string
	payload := RequestPayload{
		Message:   message,
		SessionID: sessionID,
	}

	if sessionID == "" {
		endpoint = baseURL + "/v1/inkeep/start-chat"
	} else {
		endpoint = baseURL + "/v1/inkeep/continue-chat"
	}

	chatResponse, err := makeHTTPRequest(ctx, endpoint, payload)
	if err != nil {
		logger.Errorf("An error occurred, ending chat: %v", err)
		return "", err
	}

	printWithFootnotes(ctx, chatResponse.Message)
	return chatResponse.SessionID, nil
}

func printWithFootnotes(ctx context.Context, text string) {
	logger := log.From(ctx)
	text = processMarkdown(text)
	// Transform footnotes
	matches := linkRegex.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		logger.Printf(text)
		return
	}

	footnotes := make(map[string]string)
	var orderedRefs []string

	for _, match := range matches {
		refName, url := match[1], match[2]
		if _, exists := footnotes[refName]; !exists {
			orderedRefs = append(orderedRefs, refName)
		}
		footnotes[refName] = url
		text = strings.Replace(text, match[0], "["+refName+"]", 1)
	}

	logger.Printf("\n%s", text)
	logger.PrintfStyled(styles.Focused, "\nReferences:")
	for _, ref := range orderedRefs {
		logger.PrintfStyled(styles.Dimmed, "[%s]: %s\n", ref, footnotes[ref])
	}
}

func handleError(logger log.Logger, err error) {
	switch e := err.(type) {
	case *sdkerrors.HTTPValidationError:
		logger.Errorf("HTTP Validation Error: %v", e)
	case *sdkerrors.SDKError:
		logger.Errorf("SDK Error: %v", e)
	default:
		logger.Errorf("Error: %v", err)
	}
}

func processMarkdown(text string) string {
	text = boldRegex.ReplaceAllStringFunc(text, func(match string) string {
		return strings.ToUpper(match[2 : len(match)-2])
	})

	text = italicRegex.ReplaceAllStringFunc(text, func(match string) string {
		return match[1 : len(match)-1]
	})

	return text
}

func RunInteractiveChatSession(ctx context.Context, message string, sessionID string) error {
	logger := log.From(ctx)
	scanner := bufio.NewScanner(os.Stdin)
	logger.Info("Entering interactive chat session, type exit or use ctrl + c to close.")
    logger.PrintfStyled(styles.Dimmed, "Example: How do I override a method name in my OpenAPI document?")

	if message != "" {
		logger.Info("\nProcessing your question, this may take a minute...")
		var err error
		sessionID, err = Ask(ctx, message, "")
		if err != nil {
			logger.Errorf("An error occurred while processing question, ending chat: %v", err)
			return err
		}
	}

	for {
		promptStyle := styles.Focused.Render("> ")
		fmt.Print(promptStyle)
		if !scanner.Scan() {
			break
		}

		input := scanner.Text()
		if input == "exit" {
			logger.Info("Exiting chat session.")
			break
		}

		var err error
        logger.Info("Processing your question, this may take a minute...")
		sessionID, err = Ask(ctx, input, sessionID)
		if err != nil {
			logger.Errorf("An error occurred: %v\n", err)
			break
		}
	}

	return nil
}

func OfferChatSessionOnError(ctx context.Context, message string) {
	logger := log.From(ctx)
	var confirm bool

	if _, err := charm_internal.NewForm(huh.NewForm(
		charm_internal.NewBranchPrompt("Would you like to enter an interactive chat session with Speakeasy AI for help?", &confirm)), fmt.Sprintf("Ask Speakeasy AI:")).
		ExecuteForm(); err != nil {
		logger.Printf("Failed to display confirmation prompt: %v", err)
		return
	}

	if confirm {
		if err := RunInteractiveChatSession(ctx, message, ""); err != nil {
			logger.Printf("Failed to start chat session: %v", err)
		}
	}
}
