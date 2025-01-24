// Code generated by Speakeasy (https://speakeasy.com). DO NOT EDIT.

package components

type TargetRunSummary struct {
	// Contents of the README file for this target
	Readme string `json:"readme"`
	// Contents of the gen.yaml file for this target
	GenYaml string `json:"gen_yaml"`
	// The path to the gen.yaml file for this target
	GenYamlPath *string `json:"gen_yaml_path,omitempty"`
	// Output directory for this target
	OutputDirectory string `json:"output_directory"`
	// Language for this target
	Language string `json:"language"`
	// Source ID in the workflow file
	SourceID string `json:"sourceID"`
	// Target ID in the workflow file
	TargetID string `json:"targetID"`
}

func (o *TargetRunSummary) GetReadme() string {
	if o == nil {
		return ""
	}
	return o.Readme
}

func (o *TargetRunSummary) GetGenYaml() string {
	if o == nil {
		return ""
	}
	return o.GenYaml
}

func (o *TargetRunSummary) GetGenYamlPath() *string {
	if o == nil {
		return nil
	}
	return o.GenYamlPath
}

func (o *TargetRunSummary) GetOutputDirectory() string {
	if o == nil {
		return ""
	}
	return o.OutputDirectory
}

func (o *TargetRunSummary) GetLanguage() string {
	if o == nil {
		return ""
	}
	return o.Language
}

func (o *TargetRunSummary) GetSourceID() string {
	if o == nil {
		return ""
	}
	return o.SourceID
}

func (o *TargetRunSummary) GetTargetID() string {
	if o == nil {
		return ""
	}
	return o.TargetID
}
