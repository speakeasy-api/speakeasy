// Code generated by Speakeasy (https://speakeasy.com). DO NOT EDIT.

package components

type RunResponseStreamEvent struct {
	// Type of the stream
	Event string `json:"event"`
	// Map of target run summaries
	Data RunResponse `json:"data"`
}

func (o *RunResponseStreamEvent) GetEvent() string {
	if o == nil {
		return ""
	}
	return o.Event
}

func (o *RunResponseStreamEvent) GetData() RunResponse {
	if o == nil {
		return RunResponse{}
	}
	return o.Data
}

func (o RunResponseStreamEvent) GetEventEncoding(event string) (string, error) {
	return "application/json", nil
}
