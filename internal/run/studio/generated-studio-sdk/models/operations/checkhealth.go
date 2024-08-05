// Code generated by Speakeasy (https://speakeasyapi.dev). DO NOT EDIT.

package operations

import (
	"github.com/speakeasy-api/speakeasy/internal/run/studio/generated-studio-sdk/models/components"
)

type CheckHealthResponse struct {
	HTTPMeta components.HTTPMetadata `json:"-"`
	// Successful response
	HealthResponse *components.HealthResponse
}

func (o *CheckHealthResponse) GetHTTPMeta() components.HTTPMetadata {
	if o == nil {
		return components.HTTPMetadata{}
	}
	return o.HTTPMeta
}

func (o *CheckHealthResponse) GetHealthResponse() *components.HealthResponse {
	if o == nil {
		return nil
	}
	return o.HealthResponse
}