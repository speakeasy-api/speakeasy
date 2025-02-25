// Code generated by Speakeasy (https://speakeasy.com). DO NOT EDIT.

package operations

import (
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk/models/components"
)

type UpdateSourceRequestBody struct {
	// The studio modifications overlay contents - this should be an overlay YAML document
	Overlay *string `json:"overlay,omitempty"`
	// The input spec for the source
	Input *string `json:"input,omitempty"`
}

func (o *UpdateSourceRequestBody) GetOverlay() *string {
	if o == nil {
		return nil
	}
	return o.Overlay
}

func (o *UpdateSourceRequestBody) GetInput() *string {
	if o == nil {
		return nil
	}
	return o.Input
}

type UpdateSourceResponse struct {
	HTTPMeta components.HTTPMetadata `json:"-"`
	// Successful response
	SourceResult *components.SourceResponseData
}

func (o *UpdateSourceResponse) GetHTTPMeta() components.HTTPMetadata {
	if o == nil {
		return components.HTTPMetadata{}
	}
	return o.HTTPMeta
}

func (o *UpdateSourceResponse) GetSourceResponse() *components.SourceResponseData {
	if o == nil {
		return nil
	}
	return o.SourceResult
}
