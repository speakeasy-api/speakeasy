package cmd

import (
	"context"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/ask"
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
            Required:    true, 
        },
    },
}


func AskFunc(ctx context.Context, flags ask.AskFlags) error {
	return ask.StartFunc(ctx, flags)
}
