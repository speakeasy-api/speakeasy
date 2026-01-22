package gram

import (
	"context"
	"fmt"
	"net/http"
	"time"

	toolsetshttp "github.com/speakeasy-api/gram/server/gen/http/toolsets/client"
	"github.com/speakeasy-api/gram/server/gen/toolsets"
	"github.com/speakeasy-api/gram/server/gen/types"
	goahttp "goa.design/goa/v3/http"
)

const (
	defaultGramHost   = "app.getgram.ai"
	defaultGramScheme = "https"
)

// ToolsetsClient wraps the Gram toolsets API
type ToolsetsClient struct {
	client *toolsets.Client
}

// NewToolsetsClient creates a new Gram toolsets API client
func NewToolsetsClient() *ToolsetsClient {
	return NewToolsetsClientWithHost(defaultGramScheme, defaultGramHost)
}

// NewToolsetsClientWithHost creates a new Gram toolsets API client with a custom host
func NewToolsetsClientWithHost(scheme, host string) *ToolsetsClient {
	httpClient := &http.Client{
		Timeout: 10 * time.Minute,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	enc := goahttp.RequestEncoder
	dec := goahttp.ResponseDecoder

	h := toolsetshttp.NewClient(scheme, host, httpClient, enc, dec, false)

	client := toolsets.NewClient(
		h.CreateToolset(),
		h.ListToolsets(),
		h.UpdateToolset(),
		h.DeleteToolset(),
		h.GetToolset(),
		h.CheckMCPSlugAvailability(),
		h.CloneToolset(),
		h.AddExternalOAuthServer(),
		h.RemoveOAuthServer(),
		h.AddOAuthProxyServer(),
	)

	return &ToolsetsClient{client: client}
}

// CreateToolsetParams contains the parameters for creating a toolset
type CreateToolsetParams struct {
	Name         string
	Description  string
	ToolUrns     []string
	ResourceUrns []string
}

// CreateToolset creates a new toolset
func (c *ToolsetsClient) CreateToolset(ctx context.Context, apiKey, projectSlug string, params CreateToolsetParams) (*types.Toolset, error) {
	var desc *string
	if params.Description != "" {
		desc = &params.Description
	}

	payload := &toolsets.CreateToolsetPayload{
		ApikeyToken:      &apiKey,
		ProjectSlugInput: &projectSlug,
		Name:             params.Name,
		Description:      desc,
		ToolUrns:         params.ToolUrns,
		ResourceUrns:     params.ResourceUrns,
	}

	result, err := c.client.CreateToolset(ctx, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to create toolset: %w", err)
	}

	return result, nil
}

// EnableToolset enables MCP for a toolset with a slug
func (c *ToolsetsClient) EnableToolset(ctx context.Context, apiKey, projectSlug, toolsetSlug, mcpSlug string) (*types.Toolset, error) {
	slug := types.Slug(toolsetSlug)
	mcp := types.Slug(mcpSlug)
	enabled := true

	payload := &toolsets.UpdateToolsetPayload{
		ApikeyToken:      &apiKey,
		ProjectSlugInput: &projectSlug,
		Slug:             slug,
		McpEnabled:       &enabled,
		McpSlug:          &mcp,
	}

	result, err := c.client.UpdateToolset(ctx, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to enable toolset: %w", err)
	}

	return result, nil
}

// MakeToolsetPublic makes a toolset public
func (c *ToolsetsClient) MakeToolsetPublic(ctx context.Context, apiKey, projectSlug, toolsetSlug string) (*types.Toolset, error) {
	slug := types.Slug(toolsetSlug)
	public := true

	payload := &toolsets.UpdateToolsetPayload{
		ApikeyToken:      &apiKey,
		ProjectSlugInput: &projectSlug,
		Slug:             slug,
		McpIsPublic:      &public,
	}

	result, err := c.client.UpdateToolset(ctx, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to make toolset public: %w", err)
	}

	return result, nil
}

// GetToolset retrieves a toolset by slug
func (c *ToolsetsClient) GetToolset(ctx context.Context, apiKey, projectSlug, toolsetSlug string) (*types.Toolset, error) {
	slug := types.Slug(toolsetSlug)

	payload := &toolsets.GetToolsetPayload{
		ApikeyToken:      &apiKey,
		ProjectSlugInput: &projectSlug,
		Slug:             slug,
	}

	result, err := c.client.GetToolset(ctx, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to get toolset: %w", err)
	}

	return result, nil
}

// ListToolsets lists all toolsets for a project
func (c *ToolsetsClient) ListToolsets(ctx context.Context, apiKey, projectSlug string) ([]*types.ToolsetEntry, error) {
	payload := &toolsets.ListToolsetsPayload{
		ApikeyToken:      &apiKey,
		ProjectSlugInput: &projectSlug,
	}

	result, err := c.client.ListToolsets(ctx, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to list toolsets: %w", err)
	}

	return result.Toolsets, nil
}

// CheckMCPSlugAvailability checks if an MCP slug is available
func (c *ToolsetsClient) CheckMCPSlugAvailability(ctx context.Context, apiKey, projectSlug, mcpSlug string) (bool, error) {
	slug := types.Slug(mcpSlug)

	payload := &toolsets.CheckMCPSlugAvailabilityPayload{
		ApikeyToken:      &apiKey,
		ProjectSlugInput: &projectSlug,
		Slug:             slug,
	}

	available, err := c.client.CheckMCPSlugAvailability(ctx, payload)
	if err != nil {
		return false, fmt.Errorf("failed to check MCP slug availability: %w", err)
	}

	return available, nil
}

// DeleteToolset deletes a toolset by slug
func (c *ToolsetsClient) DeleteToolset(ctx context.Context, apiKey, projectSlug, toolsetSlug string) error {
	slug := types.Slug(toolsetSlug)

	payload := &toolsets.DeleteToolsetPayload{
		ApikeyToken:      &apiKey,
		ProjectSlugInput: &projectSlug,
		Slug:             slug,
	}

	err := c.client.DeleteToolset(ctx, payload)
	if err != nil {
		return fmt.Errorf("failed to delete toolset: %w", err)
	}

	return nil
}
