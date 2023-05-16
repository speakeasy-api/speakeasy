package suggestions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const timeout = time.Minute * 2

var baseURL = os.Getenv("SPEAKEASY_SERVER_URL")

type suggestionResponse struct {
	Suggestion string `json:"suggestion"`
}

type suggestionRequest struct {
	Error      string `json:"error"`
	LineNumber int    `json:"line_number"`
}

func Upload(filePath string) (string, error) {
	openAIKey, err := getOpenAIKey()
	if err != nil {
		return "", err
	}

	apiKey, err := getSpeakeasyAPIKey()
	if err != nil {
		return "", err
	}

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreatePart(map[string][]string{
		"Content-Disposition": {"form-data; name=\"file\"; filename=\"" + filepath.Base(filePath) + "\""},
		"Content-Type":        {detectFileType(filePath)}, // Set the MIME type here
	})
	if err != nil {
		return "", err
	}

	_, err = part.Write(fileData)
	if err != nil {
		return "", err
	}

	err = writer.Close()
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", baseURL+"/v1/llm/openapi", body)
	if err != nil {
		return "", fmt.Errorf("error creating request for upload: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("x-openai-key", openAIKey)
	req.Header.Set("x-api-key", apiKey)

	client := &http.Client{
		Timeout: timeout,
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request for upload: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %v", resp.StatusCode)
	}

	token := resp.Header.Get("x-session-token")
	if token == "" {
		return "", fmt.Errorf("session token is empty")
	}

	return token, nil
}

func Suggestion(token string, error string, lineNumber int) (string, error) {
	openAIKey, err := getOpenAIKey()
	if err != nil {
		return "", err
	}

	apiKey, err := getSpeakeasyAPIKey()
	if err != nil {
		return "", err
	}

	reqBody := suggestionRequest{
		Error:      error,
		LineNumber: lineNumber,
	}

	jsonPayload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error marshaling request payload: %v", err)
	}

	req, err := http.NewRequest("POST", baseURL+"/v1/llm/openapi/suggestion", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("error creating request for suggest: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-session-token", token)
	req.Header.Set("x-openai-key", openAIKey)
	req.Header.Set("x-api-key", apiKey)

	client := &http.Client{
		Timeout: timeout,
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request for suggest: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %v", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var response suggestionResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("error unmarshaling response body: %v", err)
	}

	return response.Suggestion, nil
}

func Clear(token string) error {
	apiKey, err := getSpeakeasyAPIKey()
	if err != nil {
		return err
	}

	req, err := http.NewRequest("DELETE", baseURL+"/v1/llm/openapi", nil)
	if err != nil {
		return fmt.Errorf("error creating request for suggest: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-session-token", token)
	req.Header.Set("x-api-key", apiKey)

	client := &http.Client{
		Timeout: timeout,
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error making request for suggest: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %v", resp.StatusCode)
	}

	return nil
}

func getOpenAIKey() (string, error) {
	key := os.Getenv("SPEAKEASY_API_KEY_OPENAI")
	if key == "" {
		return "", fmt.Errorf("SPEAKEASY_API_KEY_OPENAI must be set")
	}

	return key, nil
}

func getSpeakeasyAPIKey() (string, error) {
	key, _ := config.GetSpeakeasyAPIKey()
	if key == "" {
		return "", fmt.Errorf("no api key available, please set SPEAKEASY_API_KEY or run 'speakeasy auth' to authenticate the CLI with the Speakeasy Platform")
	}

	return key, nil
}
