package log

type Msg struct {
	Msg  string
	Type MsgType
	Step *StepMsg // Only set when Type == MsgStep
}

type MsgType string

var (
	MsgInfo        MsgType = "info"
	MsgWarn        MsgType = "warn"
	MsgError       MsgType = "error"
	MsgGithub      MsgType = "github"
	MsgStudio      MsgType = "studio"
	MsgStepSkipped MsgType = "step_skipped"
	MsgStep        MsgType = "step"
)

// StepStatus represents the outcome of a progress step.
type StepStatus string

const (
	StepStatusPending StepStatus = "pending"
	StepStatusSuccess StepStatus = "success"
	StepStatusFailed  StepStatus = "failed"
	StepStatusSkipped StepStatus = "skipped"
)

// StepMsg contains step tracking information for MsgStep messages.
type StepMsg struct {
	ID     string     // Unique ID for this step (for updates)
	Path   []string   // Hierarchy path: ["Parent", "Child", "Grandchild"]
	Status StepStatus // Current status of the step
}
