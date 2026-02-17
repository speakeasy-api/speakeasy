package actions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/speakeasy-api/speakeasy/internal/ci/environment"
	"gopkg.in/yaml.v3"
)

const defaultAPIURL = "https://api.prod.speakeasyapi.dev"

type logProxyLevel string

const (
	logProxyLevelInfo  logProxyLevel = "info"
	logProxyLevelError logProxyLevel = "error"
)

type logProxyEntry struct {
	LogLevel logProxyLevel          `json:"log_level"`
	Message  string                 `json:"message"`
	Source   string                 `json:"source"`
	Tags     map[string]interface{} `json:"tags"`
}

func LogActionResult() error {
	serverURL := defaultAPIURL
	if s := os.Getenv("SPEAKEASY_SERVER_URL"); s != "" {
		serverURL = s
	}

	key := os.Getenv("SPEAKEASY_API_KEY")
	if key == "" {
		fmt.Print("no SPEAKEASY_API_KEY provided.")
		return nil
	}

	logLevel := logProxyLevelInfo
	logMessage := "Success in Github Action"
	if !strings.Contains(strings.ToLower(os.Getenv("GH_ACTION_RESULT")), "success") {
		logLevel = logProxyLevelError
		logMessage = "Failure in Github Action"
	}

	request := logProxyEntry{
		LogLevel: logLevel,
		Message:  logMessage,
		Source:   "gh_action",
		Tags: map[string]interface{}{
			"gh_repository":     fmt.Sprintf("https://github.com/%s", os.Getenv("GITHUB_REPOSITORY")),
			"gh_action_version": os.Getenv("GH_ACTION_VERSION"),
			"gh_action_step":    os.Getenv("GH_ACTION_STEP"),
			"gh_action_result":  os.Getenv("GH_ACTION_RESULT"),
			"gh_action_run":     fmt.Sprintf("https://github.com/%s/actions/runs/%s", os.Getenv("GITHUB_REPOSITORY"), os.Getenv("GITHUB_RUN_ID")),
			"run_origin":        "gh_action",
		},
	}

	languages := environment.GetLanguages()
	languages = strings.ReplaceAll(languages, "\\n", "\n")
	langs := []string{}
	if err := yaml.Unmarshal([]byte(languages), &langs); err != nil {
		fmt.Println("No language provided in github actions config.")
	}
	if len(langs) > 0 {
		request.Tags["language"] = langs[0]
	}

	target := os.Getenv("TARGET_TYPE")
	if len(langs) > 0 {
		if langs[0] == "docs" {
			target = "docs"
		} else {
			target = "sdk"
		}
	}
	if target != "" {
		request.Tags["target_type"] = target
	}

	if os.Getenv("GITHUB_REPOSITORY") != "" {
		request.Tags["gh_organization"] = strings.Split(os.Getenv("GITHUB_REPOSITORY"), "/")[0]
	}

	if os.Getenv("RESOLVED_SPEAKEASY_VERSION") != "" {
		request.Tags["speakeasy_version"] = os.Getenv("RESOLVED_SPEAKEASY_VERSION")
	}

	body, err := json.Marshal(&request)
	if err != nil {
		fmt.Print("failure sending log to speakeasy.")
		return nil
	}

	baseURL := os.Getenv("SPEAKEASY_SERVER_URL")
	if baseURL == "" {
		baseURL = serverURL
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
