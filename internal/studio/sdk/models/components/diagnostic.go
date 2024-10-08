// Code generated by Speakeasy (https://speakeasy.com). DO NOT EDIT.

package components

type Diagnostic struct {
	// Message describing the issue
	Message string `json:"message"`
	// Severity
	Severity string `json:"severity"`
	// Line number
	Line *int64 `json:"line,omitempty"`
	// Schema path to the issue
	Path []string `json:"path,omitempty"`
	// Issue type
	Type string `json:"type"`
	// Help message for how to fix the issue
	HelpMessage *string `json:"helpMessage,omitempty"`
}

func (o *Diagnostic) GetMessage() string {
	if o == nil {
		return ""
	}
	return o.Message
}

func (o *Diagnostic) GetSeverity() string {
	if o == nil {
		return ""
	}
	return o.Severity
}

func (o *Diagnostic) GetLine() *int64 {
	if o == nil {
		return nil
	}
	return o.Line
}

func (o *Diagnostic) GetPath() []string {
	if o == nil {
		return nil
	}
	return o.Path
}

func (o *Diagnostic) GetType() string {
	if o == nil {
		return ""
	}
	return o.Type
}

func (o *Diagnostic) GetHelpMessage() *string {
	if o == nil {
		return nil
	}
	return o.HelpMessage
}
