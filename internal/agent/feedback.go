package agent

import (
	"context"
	"fmt"

	"github.com/speakeasy-api/speakeasy-core/feedback"
)

const maxFeedbackLength = 3000

func SubmitFeedback(ctx context.Context, feedbackType, message, contextPath string) error {
	if message == "" {
		return fmt.Errorf("--message is required")
	}

	if len(message) > maxFeedbackLength {
		partsNeeded := (len(message) + maxFeedbackLength - 1) / maxFeedbackLength
		return fmt.Errorf("feedback message must be less than %d characters (got %d). Please split into %d smaller messages (e.g., part 1/%d, part 2/%d)", maxFeedbackLength, len(message), partsNeeded, partsNeeded, partsNeeded)
	}

	ft := feedback.FeedbackTypeAgentContext
	if feedbackType == "general" {
		ft = feedback.FeedbackTypeGeneral
	}

	metadata := map[string]any{}
	if contextPath != "" {
		metadata["context_path"] = contextPath
	}

	feedback.RecordFeedback(ctx, ft, message, metadata)

	fmt.Println("Feedback submitted. Thank you!")
	return nil
}
