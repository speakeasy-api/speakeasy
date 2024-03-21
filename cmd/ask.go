package cmd

import (
	"context"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/ask"
    "bufio"
    "fmt"
    "os"
    "github.com/speakeasy-api/speakeasy/internal/charm/styles"

	"github.com/speakeasy-api/speakeasy/internal/log"
)

var AskCmd = &model.ExecutableCommand[ask.AskFlags]{
    Usage:        "ask",
    Short:        "Ask a question",
    Long:         "Use this command to ask a question with the --message flag.",
    Run:          AskFunc,
    RequiresAuth: false, 
    Flags: []flag.Flag{
        flag.StringFlag{
            Name:        "message",
            Shorthand:   "m",
            Description: "Your question",
            Required:    false, 
        },
    },
}

func AskFunc(ctx context.Context, initialFlags ask.AskFlags) error {
    logger := log.From(ctx)
	sessionID := "" 
	scanner := bufio.NewScanner(os.Stdin)
    logger.PrintfStyled(styles.Focused, "Entering interactive chat session, type exit to quit.")

	if initialFlags.Message != "" {
		logger.PrintfStyled(styles.Focused, "\nProcessing your question...")
		var err error
		sessionID, err = ask.Ask(ctx, initialFlags)
		if err != nil {
			fmt.Printf("An error occurred: %v\n", err)
			// Decide whether to exit or continue
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
            logger.PrintfStyled(styles.Focused, "Exiting chat session.")
			break
		}

		flags := ask.AskFlags{
			Message:   input,
			SessionID: sessionID,
		}

		var err error
		sessionID, err = ask.Ask(ctx, flags)
		if err != nil {
			fmt.Printf("An error occurred: %v\n", err)
			break
		}
	}

	return nil
}