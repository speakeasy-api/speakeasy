// Code generated by Speakeasy (https://speakeasy.com). DO NOT EDIT.

package operations

import (
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk/models/components"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk/types/stream"
)

type RunResponse struct {
	HTTPMeta components.HTTPMetadata `json:"-"`
	// Successful response
	RunResponse *stream.EventStream[components.RunResponse]
}

func (o *RunResponse) GetHTTPMeta() components.HTTPMetadata {
	if o == nil {
		return components.HTTPMetadata{}
	}
	return o.HTTPMeta
}

func (o *RunResponse) GetRunResponse() *stream.EventStream[components.RunResponse] {
	if o == nil {
		return nil
	}
	return o.RunResponse
}
