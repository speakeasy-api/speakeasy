package ask

import (
	"context"
	"fmt"
	"regexp"
	"strings"
    "bufio"
    "os"

	aiapigo "github.com/inkeep/ai-api-go"
	"github.com/inkeep/ai-api-go/models/components"
	"github.com/inkeep/ai-api-go/models/sdkerrors"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"

	"github.com/speakeasy-api/speakeasy/internal/log"
)

type AskFlags struct {
	Message string `json:"message"`
	SessionID string `json:"sessionId,omitempty"`
}


var (
    boldRegex   = regexp.MustCompile(`\*\*(.*?)\*\*`)
    italicRegex = regexp.MustCompile(`\*(.*?)\*`)
    linkRegex   = regexp.MustCompile(`\[\(?(.*?)\)?\]\((https?:\/\/[^\s]+)\)`)
)


func Ask(ctx context.Context, flags AskFlags) (string, error) {
	logger := log.From(ctx)
	s := aiapigo.New(aiapigo.WithSecurity(os.Getenv("INKEEP_API_KEY")))
    var SessionID string
	if flags.SessionID == "" {
        res, err := s.ChatSession.Create(ctx, components.CreateChatSessionWithChatResultInput{
            IntegrationID: os.Getenv("INKEEP_INTEGRATION_ID"),
            ChatSession: components.ChatSessionInput{
                Messages: []components.Message{{
                    UserMessage: &components.UserMessage{
                        Role:    "user",
                        Content: flags.Message,
                    },
                }},
            },
        })
		if err != nil {
            SessionID = res.ChatResult.ChatSessionID
			handleError(logger, err)
			return SessionID, err
		}

		if res.ChatResult != nil {
            printWithFootnotes(ctx, res.ChatResult.Message.Content)
        } else {
            logger.Error("\nNo response received.")
        }
	} else {
		res, err := s.ChatSession.Continue(ctx, flags.SessionID, components.ContinueChatSessionWithChatResultInput{
			IntegrationID: os.Getenv("INKEEP_INTEGRATION_ID"),
            Message: components.Message{
                AssistantMessage: &components.AssistantMessage{
                    Content: flags.Message,
                },
            },
		})
		if err != nil {
            SessionID = res.ChatResult.ChatSessionID
			handleError(logger, err)
			return SessionID, err
		}

        if res.ChatResult != nil {
            printWithFootnotes(ctx, res.ChatResult.Message.Content)
        } else {
            logger.Error("\nNo chat response received.")
        }
	}

	return SessionID, nil 
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
        // Directly replace the markdown link with the reference name, removing parentheses in the process
        text = strings.Replace(text, match[0], "["+refName+"]", 1)
    }

    logger.Printf("\n%s",text)
    logger.PrintfStyled(styles.Focused, "\nReferences:")
    for _, ref := range orderedRefs {
        logger.PrintfStyled(styles.Dimmed, "[%s]: %s\n", ref, footnotes[ref])
    }
}



func RunInteractiveChatSession(ctx context.Context, initialFlags AskFlags) error {
    logger := log.From(ctx)
	sessionID := "" 
	scanner := bufio.NewScanner(os.Stdin)
    logger.Info("Entering interactive chat session, type exit to quit.")

	if initialFlags.Message != "" {
		logger.Info("\nProcessing your question...")
		var err error
		sessionID, err = Ask(ctx, initialFlags)
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

		flags := AskFlags{
			Message:   input,
			SessionID: sessionID,
		}

		var err error
		sessionID, err = Ask(ctx, flags)
		if err != nil {
            logger.Errorf("An error occurred: %v\n", err)
			break
		}
	}

	return nil
}

// OfferChatSessionOnError offers the user to enter an interactive chat session if an error occurs.
func OfferChatSessionOnError(ctx context.Context, message string) {
    logger := log.From(ctx) 

    logger.PrintfStyled(styles.Focused, "Would you like to enter an interactive chat session with Speakeasy AI for help? (yes/no):")
    scanner := bufio.NewScanner(os.Stdin)
    if scanner.Scan() {
        input := scanner.Text()
        if input == "yes" || input == "y" {
            initialFlags := AskFlags{
                Message: message, 
            }
            if err := RunInteractiveChatSession(ctx, initialFlags); err != nil {
                logger.Errorf("Failed to start chat session: %v", err)
            }
        }
    }
}