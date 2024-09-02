// Code generated by Speakeasy (https://speakeasy.com). DO NOT EDIT.

package operations

import (
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk/models/components"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk/types/stream"
)

type GetRunResponse struct {
	HTTPMeta components.HTTPMetadata `json:"-"`
	// Successful response
	RunResponseStreamEvent *stream.EventStream[components.RunResponseStreamEvent]
}

func (o *GetRunResponse) GetHTTPMeta() components.HTTPMetadata {
	if o == nil {
		return components.HTTPMetadata{}
	}
	return o.HTTPMeta
}

func (o *GetRunResponse) GetRunResponseStreamEvent() *stream.EventStream[components.RunResponseStreamEvent] {
	if o == nil {
		return nil
	}
	return o.RunResponseStreamEvent
}
