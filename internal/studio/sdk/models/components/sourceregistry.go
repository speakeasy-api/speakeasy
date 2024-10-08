// Code generated by Speakeasy (https://speakeasy.com). DO NOT EDIT.

package components

type SourceRegistry struct {
	// Source registry location
	Location *string `json:"location,omitempty"`
	// List of tags
	Tags []string `json:"tags,omitempty"`
}

func (o *SourceRegistry) GetLocation() *string {
	if o == nil {
		return nil
	}
	return o.Location
}

func (o *SourceRegistry) GetTags() []string {
	if o == nil {
		return nil
	}
	return o.Tags
}
