package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	speakeasy "github.com/speakeasy-api/speakeasy-client-sdk-go/v3"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy/internal/ci/environment"
)

type ContextKey string

const (
	ExecutionKeyEnvironmentVariable            = "SPEAKEASY_EXECUTION_ID"
	SpeakeasySDKKey                 ContextKey = "speakeasy.SDK"
	WorkspaceIDKey                  ContextKey = "speakeasy.workspaceID"
	AccountTypeKey                  ContextKey = "speakeasy.accountType"
	WorkspaceSlugKey                ContextKey = "speakeasy.workspaceSlug"
	OrgSlugKey                      ContextKey = "speakeasy.orgSlug"
)

// a random UUID. Change this to fan-out executions with the same gh run id.
const speakeasyGithubActionNamespace = "360D564A-5583-4EF6-BC2B-99530BF036CC"
const speakeasyAudience = "speakeasy-generation"

func NewContextWithSDK(ctx context.Context, apiKey string) (context.Context, *speakeasy.Speakeasy, string, error) {
	security := shared.Security{APIKey: &apiKey}

	opts := []speakeasy.SDKOption{speakeasy.WithSecurity(security)}
	if os.Getenv("SPEAKEASY_SERVER_URL") != "" {
		opts = append(opts, speakeasy.WithServerURL(os.Getenv("SPEAKEASY_SERVER_URL")))
	}

	sdk := speakeasy.New(opts...)
	validated, err := sdk.Auth.ValidateAPIKey(ctx)
	if err != nil {
		return ctx, nil, "", err
	}
	sdkWithWorkspace := speakeasy.New(speakeasy.WithSecurity(security), speakeasy.WithWorkspaceID(validated.APIKeyDetails.WorkspaceID))
	ctx = context.WithValue(ctx, SpeakeasySDKKey, sdkWithWorkspace)
	ctx = context.WithValue(ctx, WorkspaceIDKey, validated.APIKeyDetails.WorkspaceID)
	ctx = context.WithValue(ctx, AccountTypeKey, validated.APIKeyDetails.AccountTypeV2)
	ctx = context.WithValue(ctx, WorkspaceSlugKey, validated.APIKeyDetails.WorkspaceSlug)
	ctx = context.WithValue(ctx, OrgSlugKey, validated.APIKeyDetails.OrgSlug)
	return ctx, sdkWithWorkspace, validated.APIKeyDetails.WorkspaceID, err
}

func GetApiKey() string {
	return os.Getenv("SPEAKEASY_API_KEY")
}

func EnrichEventWithEnvironmentVariables(event *shared.CliEvent) {
	if os.Getenv("GITHUB_ACTIONS") != "true" {
		return
	}
	ghActionOrg := os.Getenv("GITHUB_REPOSITORY_OWNER")
	ghActionRepoOrg := os.Getenv("GITHUB_REPOSITORY")
	event.GhActionOrganization = &ghActionOrg
	repo := strings.TrimPrefix(ghActionRepoOrg, ghActionOrg+"/")
	event.GhActionRepository = &repo
	runLink := fmt.Sprintf("%s/%s/actions/runs/%s", os.Getenv("GITHUB_SERVER_URL"), ghActionRepoOrg, os.Getenv("GITHUB_RUN_ID"))
	event.GhActionRunLink = &runLink

	ghActionVersion := os.Getenv("GH_ACTION_VERSION")
	if ghActionVersion != "" {
		event.GhActionVersion = &ghActionVersion
	}
}

func enrichHostName(event *shared.CliEvent) {
	hostname, err := os.Hostname()
	if err != nil {
		return
	}
	event.Hostname = &hostname
}

func Track(ctx context.Context, exec shared.InteractionType, fn func(ctx context.Context, event *shared.CliEvent) error) error {
	// Generate a unique ID for this event
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}

	runID := os.Getenv("GITHUB_RUN_ID")
	if runID == "" {
		return fmt.Errorf("no GITHUB_RUN_ID provided")
	}
	runAttempt := os.Getenv("GITHUB_RUN_ATTEMPT")
	if runAttempt == "" {
		return fmt.Errorf("no GITHUB_RUN_ATTEMPT provided")
	}
	executionKey := fmt.Sprintf("GITHUB_RUN_ID_%s, GITHUB_RUN_ATTEMPT_%s", runID, runAttempt)
	namespace, err := uuid.Parse(speakeasyGithubActionNamespace)
	if err != nil {
		return err
	}

	apiKey := GetApiKey()
	if apiKey == "" {
		return fmt.Errorf("no SPEAKEASY_API_KEY secret provided")
	}
	ctx, sdk, workspaceID, err := NewContextWithSDK(ctx, apiKey)
	if err != nil {
		return err
	}
	executionID := uuid.NewSHA1(namespace, []byte(executionKey)).String()
	_ = os.Setenv(ExecutionKeyEnvironmentVariable, executionID)

	// Prepare the initial CliEvent
	runEvent := &shared.CliEvent{
		CreatedAt:        time.Now(),
		ExecutionID:      executionID,
		ID:               id.String(),
		WorkspaceID:      workspaceID,
		InteractionType:  exec,
		LocalStartedAt:   time.Now(),
		SpeakeasyVersion: os.Getenv("GH_ACTION_VERSION"),
		Success:          false,
	}
	runEvent.WorkspaceID = workspaceID

	EnrichEventWithEnvironmentVariables(runEvent)
	enrichHostName(runEvent)

	// This means we have `id-token: write` permissions. Authenticate the workflow run with speakeasy
	if environment.GetGithubOIDCRequestURL() != "" && environment.GetGithubOIDCRequestToken() != "" {
		go func() {
			if oidcToken, err := getIDToken(environment.GetGithubOIDCRequestURL(), environment.GetGithubOIDCRequestToken()); err != nil {
				fmt.Println("Failed to get OIDC token", err)
			} else {
				owner := os.Getenv("GITHUB_REPOSITORY_OWNER")
				res, err := sdk.Github.LinkGithub(context.WithoutCancel(ctx), operations.LinkGithubAccessRequest{
					GithubOidcToken: &oidcToken,
					GithubOrg:       &owner,
				})
				if err != nil {
					fmt.Println("Failed to link github account", err)
				}

				if res != nil && res.StatusCode != http.StatusOK {
					fmt.Println("Failed to link github account", err)
				}
			}
		}()
	}

	// Execute the provided function, capturing any error
	err = fn(ctx, runEvent)

	// Populate event with pull request env var (available only after run)
	ghPullRequest := reformatPullRequestURL(os.Getenv("GH_PULL_REQUEST"))

	if ghPullRequest != "" {
		runEvent.GhPullRequest = &ghPullRequest
	}

	// Update the event with completion details
	curTime := time.Now()
	runEvent.LocalCompletedAt = &curTime
	duration := runEvent.LocalCompletedAt.Sub(runEvent.LocalStartedAt).Milliseconds()
	runEvent.DurationMs = &duration

	// For publishing events runEvent success is set by publishEvent.go
	if exec != shared.InteractionTypePublish {
		runEvent.Success = err == nil
	}
	currentIntegrationEnvironment := "GITHUB_ACTIONS"
	runEvent.ContinuousIntegrationEnvironment = &currentIntegrationEnvironment

	// Attempt to flush any stored events (swallow errors)
	_, _ = sdk.Events.Post(ctx, operations.PostWorkspaceEventsRequest{
		RequestBody: []shared.CliEvent{*runEvent},
		WorkspaceID: &workspaceID,
	})

	return err

}

// Reformat from  https://api.github.com/repos/.../.../pulls/... to https://github.com/.../.../pull/...
func reformatPullRequestURL(url string) string {
	url = strings.Replace(url, "https://api.github.com/repos/", "https://github.com/", 1)
	return strings.Replace(url, "/pulls/", "/pull/", 1)
}

type OIDCTokenResponse struct {
	Value string `json:"value"`
}

func getIDToken(requestURL string, requestToken string) (string, error) {
	tokenURL, err := url.Parse(requestURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse token request URL: %w", err)
	}

	q := tokenURL.Query()
	q.Set("audience", speakeasyAudience)
	tokenURL.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, tokenURL.String(), nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+requestToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to retrieve token, status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var tokenResponse OIDCTokenResponse
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return tokenResponse.Value, nil
}
