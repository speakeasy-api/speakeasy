// Code generated by Speakeasy (https://speakeasy.com). DO NOT EDIT.

package components

// SuggestResponse - Successful response
type SuggestResponse struct {
	// The studio modifications overlay contents - this should be an overlay YAML document
	Overlay string `json:"overlay"`
}

func (o *SuggestResponse) GetOverlay() string {
	if o == nil {
		return ""
	}
	return o.Overlay
}
