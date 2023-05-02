package suggestions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

const baseURL = "http://localhost:5050"

type suggestionResponse struct {
	Suggestion string `json:"suggestion"`
}

type suggestionRequest struct {
	Error      string `json:"error"`
	LineNumber int    `json:"line_number"`
}

func Upload() (string, error) {
	req, err := http.NewRequest("POST", baseURL+"/upload", nil)
	if err != nil {
		return "", fmt.Errorf("error creating request for upload: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
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

func Suggest(token string, error string, lineNumber int) (string, error) {
	reqBody := suggestionRequest{
		Error:      error,
		LineNumber: lineNumber,
	}

	jsonPayload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error marshaling request payload: %v", err)
	}

	req, err := http.NewRequest("POST", baseURL+"/suggest", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("error creating request for suggest: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-session-token", token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request for suggest: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %v", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)

	var response suggestionResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("error unmarshaling response body: %v", err)
	}

	return response.Suggestion, nil
}
