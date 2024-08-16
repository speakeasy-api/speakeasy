// Code generated by Speakeasy (https://speakeasy.com). DO NOT EDIT.

package components

type Source struct {
	// List of input documents
	Inputs []Document `json:"inputs,omitempty"`
	// List of overlays
	Overlays []Overlay `json:"overlays,omitempty"`
	// Output string
	Output *string `json:"output,omitempty"`
	// Ruleset string
	Ruleset  *string         `json:"ruleset,omitempty"`
	Registry *SourceRegistry `json:"registry,omitempty"`
}

func (o *Source) GetInputs() []Document {
	if o == nil {
		return nil
	}
	return o.Inputs
}

func (o *Source) GetOverlays() []Overlay {
	if o == nil {
		return nil
	}
	return o.Overlays
}

func (o *Source) GetOutput() *string {
	if o == nil {
		return nil
	}
	return o.Output
}

func (o *Source) GetRuleset() *string {
	if o == nil {
		return nil
	}
	return o.Ruleset
}

func (o *Source) GetRegistry() *SourceRegistry {
	if o == nil {
		return nil
	}
	return o.Registry
}
