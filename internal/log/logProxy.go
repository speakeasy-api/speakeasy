package log

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultAPIURL = "https://api.prod.speakeasy.com"

type logProxyLevel string

const (
	LogProxyLevelInfo  logProxyLevel = "info"
	LogProxyLevelError logProxyLevel = "error"
)

type logProxyEntry struct {
	LogLevel logProxyLevel          `json:"log_level"`
	Message  string                 `json:"message"`
	Source   string                 `json:"source"`
	Tags     map[string]interface{} `json:"tags"`
}

func SendToLogProxy(ctx context.Context, logLevel logProxyLevel, logMessage string, tags map[string]interface{}) error {
	key := config.GetSpeakeasyAPIKey()
	if key == "" {
		return fmt.Errorf("SPEAKEASY_API_KEY not found")
	}

	request := logProxyEntry{
		LogLevel: logLevel,
		Message:  logMessage,
	}

	if env.IsGithubAction() {
		request.Source = "gh_action"
		request.Tags = map[string]interface{}{
			"gh_repository":     fmt.Sprintf("https://github.com/%s", os.Getenv("GITHUB_REPOSITORY")),
			"gh_action_version": os.Getenv("GH_ACTION_VERSION"),
			"gh_action_step":    os.Getenv("GH_ACTION_STEP"),
			"gh_action_result":  os.Getenv("GH_ACTION_RESULT"),
			"gh_action_run":     fmt.Sprintf("https://github.com/%s/actions/runs/%s", os.Getenv("GITHUB_REPOSITORY"), os.Getenv("GITHUB_RUN_ID")),
			"run_origin":        "gh_action",
		}
	} else {
		request.Source = "local"
		request.Tags = map[string]interface{}{
			"run_origin": "local",
		}
	}

	for k, v := range tags {
		request.Tags[k] = v
	}

	if os.Getenv("GITHUB_REPOSITORY") != "" {
		request.Tags["gh_organization"] = strings.Split(os.Getenv("GITHUB_REPOSITORY"), "/")[0]
	}

	request.Tags["speakeasy_version"] = events.GetSpeakeasyVersionFromContext(ctx)

	body, err := json.Marshal(&request)
	if err != nil {
		fmt.Print("failure sending log to speakeasy.")
		return nil
	}

	baseURL := os.Getenv("SPEAKEASY_SERVER_URL")
	if baseURL == "" {
		baseURL = defaultAPIURL
	}

	req, err := http.NewRequest("POST", baseURL+"/v1/log/proxy", bytes.NewBuffer(body))
	if err != nil {
		fmt.Print("failure sending log to speakeasy.")
		return nil
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", key)

	client := &http.Client{
		Timeout: time.Second * 5,
	}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Print("failure sending log to speakeasy.")
		return nil
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("failure sending log to speakeasy with status %s.", resp.Status)
	}

	return nil
}
