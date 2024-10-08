// Code generated by Speakeasy (https://speakeasy.com). DO NOT EDIT.

package components

type Target struct {
	// Target language
	Target string `json:"target"`
	// Source ID
	Source string `json:"source"`
	// Output string
	Output      *string      `json:"output,omitempty"`
	Publishing  *Publishing  `json:"publishing,omitempty"`
	CodeSamples *CodeSamples `json:"codeSamples,omitempty"`
}

func (o *Target) GetTarget() string {
	if o == nil {
		return ""
	}
	return o.Target
}

func (o *Target) GetSource() string {
	if o == nil {
		return ""
	}
	return o.Source
}

func (o *Target) GetOutput() *string {
	if o == nil {
		return nil
	}
	return o.Output
}

func (o *Target) GetPublishing() *Publishing {
	if o == nil {
		return nil
	}
	return o.Publishing
}

func (o *Target) GetCodeSamples() *CodeSamples {
	if o == nil {
		return nil
	}
	return o.CodeSamples
}
