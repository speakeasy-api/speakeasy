package cmd

import (
	"context"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/ask"
)

var AskCmd = &model.ExecutableCommand[ask.AskFlags]{
    Usage:        "ask",
    Short:        "Ask AI",
    Long:         "Starts a conversation with Speakeasy trained AI.",
    Run:          AskFunc,
    RequiresAuth: false, 
    Flags: []flag.Flag{
        flag.StringFlag{
            Name:        "message",
            Shorthand:   "m",
            Description: "Your question for AI.",
            Required:    false, 
        },
    },
}


func AskFunc(ctx context.Context, flags ask.AskFlags) error {
	return ask.RunInteractiveChatSession(ctx, flags)
}
