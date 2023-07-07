package suggestions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const uploadTimeout = time.Minute * 2
const suggestionTimeout = time.Minute * 1

const ApiURL = "https://api.prod.speakeasyapi.dev"

var baseURL = ApiURL

type suggestionResponse struct {
	Suggestion string `json:"suggestion"`
}

type suggestionRequest struct {
	Error      string `json:"error"`
	LineNumber int    `json:"line_number"`
}

func Upload(filePath string) (string, string, error) {
	openAIKey, err := GetOpenAIKey()
	if err != nil {
		return "", "", err
	}

	apiKey, err := getSpeakeasyAPIKey()
	if err != nil {
		return "", "", err
	}

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return "", "", err
	}

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreatePart(map[string][]string{
		"Content-Disposition": {"form-data; name=\"file\"; filename=\"" + filepath.Base(filePath) + "\""},
		"Content-Type":        {detectFileType(filePath)}, // Set the MIME type here
	})
	if err != nil {
		return "", "", err
	}

	_, err = part.Write(fileData)
	if err != nil {
		return "", "", err
	}

	err = writer.Close()
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequest("POST", baseURL+"/v1/llm/openapi", body)
	if err != nil {
		return "", "", fmt.Errorf("error creating request for upload: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("x-openai-key", openAIKey)
	req.Header.Set("x-api-key", apiKey)

	client := &http.Client{
		Timeout: uploadTimeout,
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("error making request for upload: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnprocessableEntity {
		return "", "", fmt.Errorf("OpenAPI document is larger than 50,000 line limit")
	}

	err = checkResponseStatusCode(resp)
	if err != nil {
		return "", "", err
	}

	token := resp.Header.Get("x-session-token")
	if token == "" {
		return "", "", fmt.Errorf("session token is empty")
	}

	return token, strings.ToLower(filepath.Ext(filePath))[1:], nil
}

func Suggestion(token string, error string, lineNumber int, fileType string, model string) (string, error) {
	openAIKey, err := GetOpenAIKey()
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
	req.Header.Set("x-file-type", fileType)
	req.Header.Set("x-openai-model", model)

	client := &http.Client{
		Timeout: suggestionTimeout,
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request for suggest: %v", err)
	}

	defer resp.Body.Close()

	err = checkResponseStatusCode(resp)
	if err != nil {
		return "", err
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
		Timeout: suggestionTimeout,
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error making request for suggest: %v", err)
	}

	defer resp.Body.Close()

	err = checkResponseStatusCode(resp)
	if err != nil {
		return err
	}

	return nil
}

func GetOpenAIKey() (string, error) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		return "", fmt.Errorf("OPENAI_API_KEY must be set to use LLM Suggestions")
	}

	return key, nil
}

func getSpeakeasyAPIKey() (string, error) {
	key, _ := config.GetSpeakeasyAPIKey()
	if key == "" {
		return "", fmt.Errorf("no speakeasy api key available, please set SPEAKEASY_API_KEY or run 'speakeasy auth' to authenticate the CLI with the Speakeasy Platform")
	}

	return key, nil
}

func checkResponseStatusCode(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK {
		var errBody []byte
		var err error
		errBody, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed reading response error body: %v", err)
		}
		return fmt.Errorf("unexpected status code: %v\nresponse status: %s\nerror: %s", resp.StatusCode, resp.Status, errBody)
	}

	return nil
}

func init() {
	if url := os.Getenv("SPEAKEASY_SERVER_URL"); url != "" {
		baseURL = url
	}
}
