package agent

import (
	"context"
	"fmt"

	"github.com/speakeasy-api/speakeasy-core/feedback"
)

func SubmitFeedback(ctx context.Context, feedbackType, message, contextPath string) error {
	if message == "" {
		return fmt.Errorf("--message is required")
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
