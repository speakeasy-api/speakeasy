// Code generated by Speakeasy (https://speakeasy.com). DO NOT EDIT.

package components

type Overlay struct {
	FallbackCodeSamples *FallbackCodeSamples `json:"fallbackCodeSamples,omitempty"`
	Document            *Document            `json:"document,omitempty"`
}

func (o *Overlay) GetFallbackCodeSamples() *FallbackCodeSamples {
	if o == nil {
		return nil
	}
	return o.FallbackCodeSamples
}

func (o *Overlay) GetDocument() *Document {
	if o == nil {
		return nil
	}
	return o.Document
}
