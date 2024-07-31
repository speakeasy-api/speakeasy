// Code generated by Speakeasy (https://speakeasyapi.dev). DO NOT EDIT.

package operations

import (
	"github.com/speakeasy-api/speakeasy/internal/run/generated-studio-sdk/models/components"
)

type RunResponse struct {
	HTTPMeta components.HTTPMetadata `json:"-"`
	// Successful response
	RunResponse *components.RunResponse
}

func (o *RunResponse) GetHTTPMeta() components.HTTPMetadata {
	if o == nil {
		return components.HTTPMetadata{}
	}
	return o.HTTPMeta
}

func (o *RunResponse) GetRunResponse() *components.RunResponse {
	if o == nil {
		return nil
	}
	return o.RunResponse
}