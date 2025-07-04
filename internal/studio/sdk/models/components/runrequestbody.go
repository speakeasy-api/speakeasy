// Code generated by Speakeasy (https://speakeasy.com). DO NOT EDIT.

package components

type RunRequestBody struct {
	// The studio modifications overlay contents - this should be an overlay YAML document
	Overlay *string `json:"overlay,omitempty"`
	// The input spec for the source
	Input *string `json:"input,omitempty"`
	// Map of target specific inputs keyed on target name
	// Only present if a target input is modified
	//
	Targets map[string]TargetSpecificInputs `json:"targets,omitempty"`
	// whether to disconnect the studio when completed
	Disconnect bool              `json:"disconnect"`
	Stream     *RunStreamOptions `json:"stream,omitempty"`
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

func (o *RunRequestBody) GetTargets() map[string]TargetSpecificInputs {
	if o == nil {
		return nil
	}
	return o.Targets
}

func (o *RunRequestBody) GetDisconnect() bool {
	if o == nil {
		return false
	}
	return o.Disconnect
}

func (o *RunRequestBody) GetStream() *RunStreamOptions {
	if o == nil {
		return nil
	}
	return o.Stream
}
