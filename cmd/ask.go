package cmd

import (
	"context"
	"github.com/speakeasy-api/speakeasy/internal/ask"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
)

type AskFlags struct {
	Message   string `json:"message"`
	SessionID string `json:"sessionID,omitempty"`
}

var AskCmd = &model.ExecutableCommand[AskFlags]{
	Usage:        "ask",
	Short:        "Starts a conversation with Speakeasy trained AI",
	Long:         "Starts a conversation with Speakeasy trained AI. Ask about OpenAPI, Speakeasy, configuring SDKs, or anything else you need help with.",
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

func AskFunc(ctx context.Context, flags AskFlags) error {
	return ask.RunInteractiveChatSession(ctx, flags.Message, flags.SessionID)
}
