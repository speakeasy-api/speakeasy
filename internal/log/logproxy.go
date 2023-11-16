package log

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/speakeasy-api/speakeasy/internal/config"
	"go.uber.org/zap"
)

const defaultAPIURL = "https://api.prod.speakeasyapi.dev"

type logProxyLevel string

const (
	logProxyLevelInfo  logProxyLevel = "info"
	logProxyLevelError logProxyLevel = "error"
)

type logProxyEntry struct {
	LogLevel logProxyLevel `json:"log_level"`
	Message  string        `json:"message"`
	// We may make more logic decisions based source at some point.
	Source string                 `json:"source"`
	Tags   map[string]interface{} `json:"tags"`
}

func flushLog(message string, level logProxyLevel, fields []zap.Field) error {
	key, _ := config.GetSpeakeasyAPIKey()
	if key == "" {
		return fmt.Errorf("no speakeasy api key available, please set SPEAKEASY_API_KEY or run 'speakeasy auth' to authenticate the CLI with the Speakeasy Platform")
	}

	request := logProxyEntry{
		LogLevel: level,
		Message:  message,
		Source:   "cli",
	}

	tags := make(map[string]interface{}, 0)
	for _, field := range fields {
		tags[field.Key] = field.Interface
	}
	request.Tags = tags

	body, err := json.Marshal(&request)
	if err != nil {
		return err
	}

	baseURL := os.Getenv("SPEAKEASY_SERVER_URL")
	if baseURL == "" {
		baseURL = defaultAPIURL
	}

	req, err := http.NewRequest("POST", baseURL+"/v1/log/proxy", bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", key)

	client := &http.Client{
		Timeout: time.Second * 5,
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("session token is empty")
	}

	return nil
}
