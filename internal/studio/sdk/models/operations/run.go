// Code generated by Speakeasy (https://speakeasy.com). DO NOT EDIT.

package operations

import (
	"errors"
	"fmt"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk/internal/utils"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk/models/components"
	"github.com/speakeasy-api/speakeasy/internal/studio/sdk/types/stream"
)

type Config struct {
	// The config edit for the source
	Data string `json:"data"`
	// The local path to config file
	Path string `json:"path"`
}

func (o *Config) GetData() string {
	if o == nil {
		return ""
	}
	return o.Data
}

func (o *Config) GetPath() string {
	if o == nil {
		return ""
	}
	return o.Path
}

type RunRequestBody struct {
	// The studio modifications overlay contents - this should be an overlay YAML document
	Overlay *string `json:"overlay,omitempty"`
	// The input spec for the source
	Input  *string `json:"input,omitempty"`
	Config *Config `json:"config,omitempty"`
}

func (o *RunRequestBody) GetOverlay() *string {
	if o == nil {
		return nil
	}
	return o.Overlay
}

func (o *RunRequestBody) GetInput() *string {
	if o == nil {
		return nil
	}
	return o.Input
}

func (o *RunRequestBody) GetConfig() *Config {
	if o == nil {
		return nil
	}
	return o.Config
}

type RunResponseBodyType string

const (
	RunResponseBodyTypeRunResponseStreamEvent RunResponseBodyType = "RunResponseStreamEvent"
)

// RunResponseBody - Successful response
type RunResponseBody struct {
	RunResponseStreamEvent *components.RunResponseStreamEvent `queryParam:"inline"`

	Type RunResponseBodyType
}

func CreateRunResponseBodyRunResponseStreamEvent(runResponseStreamEvent components.RunResponseStreamEvent) RunResponseBody {
	typ := RunResponseBodyTypeRunResponseStreamEvent

	return RunResponseBody{
		RunResponseStreamEvent: &runResponseStreamEvent,
		Type:                   typ,
	}
}

func (u *RunResponseBody) UnmarshalJSON(data []byte) error {

	var runResponseStreamEvent components.RunResponseStreamEvent = components.RunResponseStreamEvent{}
	if err := utils.UnmarshalJSON(data, &runResponseStreamEvent, "", true, true); err == nil {
		u.RunResponseStreamEvent = &runResponseStreamEvent
		u.Type = RunResponseBodyTypeRunResponseStreamEvent
		return nil
	}

	return fmt.Errorf("could not unmarshal `%s` into any supported union types for RunResponseBody", string(data))
}

func (u RunResponseBody) MarshalJSON() ([]byte, error) {
	if u.RunResponseStreamEvent != nil {
		return utils.MarshalJSON(u.RunResponseStreamEvent, "", true)
	}

	return nil, errors.New("could not marshal union type RunResponseBody: all fields are null")
}

type RunResponse struct {
	HTTPMeta components.HTTPMetadata `json:"-"`
	// Successful response
	OneOf *stream.EventStream[RunResponseBody]
}

func (o *RunResponse) GetHTTPMeta() components.HTTPMetadata {
	if o == nil {
		return components.HTTPMetadata{}
	}
	return o.HTTPMeta
}

func (o *RunResponse) GetOneOf() *stream.EventStream[RunResponseBody] {
	if o == nil {
		return nil
	}
	return o.OneOf
}
