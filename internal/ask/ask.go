package ask

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	aiapigo "github.com/inkeep/ai-api-go"
	"github.com/inkeep/ai-api-go/models/components"
	"github.com/inkeep/ai-api-go/models/sdkerrors"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"

	"github.com/speakeasy-api/speakeasy/internal/log"
)

const (
	bearerToken   = ""
	integrationID = ""
)

type AskFlags struct {
	Message string `json:"message"`
	SessionID string `json:"sessionId,omitempty"`
}

func Ask(ctx context.Context, flags AskFlags) (string, error) {
	logger := log.From(ctx)
	s := aiapigo.New(aiapigo.WithSecurity(bearerToken))
    var SessionID string
	if flags.SessionID == "" {
        res, err := s.ChatSession.Create(ctx, components.CreateChatSessionWithChatResultInput{
            IntegrationID: integrationID,
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
            fmt.Println("\nNo response received.")
        }
	} else {
		res, err := s.ChatSession.Continue(ctx, flags.SessionID, components.ContinueChatSessionWithChatResultInput{
			IntegrationID: integrationID,
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
            fmt.Println("\nNo response received.")
        }
	}

	return SessionID, nil 
}


func handleError(logger log.Logger, err error) {
	switch e := err.(type) {
	case *sdkerrors.HTTPValidationError:
		logger.Printf("HTTP Validation Error: %v", e)
	case *sdkerrors.SDKError:
		logger.Printf("SDK Error: %v", e)
	default:
		logger.Printf("Error: %v", err)
	}
}

func printWithFootnotes(ctx context.Context, text string) {
	logger := log.From(ctx)
    // Handle bold by removing ** 
    boldRegex := regexp.MustCompile(`\*\*(.*?)\*\*`)
    text = boldRegex.ReplaceAllStringFunc(text, func(match string) string {
        return strings.ToUpper(match[2 : len(match)-2])
    })

    // Handle italic by removing *
    italicRegex := regexp.MustCompile(`\*(.*?)\*`)
    text = italicRegex.ReplaceAllStringFunc(text, func(match string) string {
        return match[1 : len(match)-1] 
    })
    
    // Transform footnotes
    linkRegex := regexp.MustCompile(`\[\(?(.*?)\)?\]\((https?:\/\/[^\s]+)\)`)
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