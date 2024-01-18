package run

import (
	"fmt"
	"github.com/speakeasy-api/speakeasy/internal/styles"
	"strings"
)

type Status string

const (
	StatusRunning   Status = "running"
	StatusFailed    Status = "failed"
	StatusSucceeded Status = "success"
)

type WorkflowStep struct {
	name     string
	status   Status
	substeps []*WorkflowStep
	nextStep *WorkflowStep
	updates  chan<- UpdateMsg
}

func NewWorkflowStep(name string, sub chan<- UpdateMsg) *WorkflowStep {
	return &WorkflowStep{
		name:     name,
		status:   StatusRunning,
		substeps: []*WorkflowStep{},
		updates:  sub,
	}
}

func (w *WorkflowStep) SetNextStep(next *WorkflowStep) {
	w.status = StatusSucceeded // If we go to the next step, we're successful
	w.nextStep = next

	w.Notify()
}

func (w *WorkflowStep) NextStep(name string) *WorkflowStep {
	next := NewWorkflowStep(name, w.updates)

	w.status = StatusSucceeded // If we go to the next step, we're successful
	w.nextStep = next

	w.Notify()

	return next
}

func (w *WorkflowStep) NextSubstep(name string) *WorkflowStep {
	substep := NewWorkflowStep(name, w.updates)

	w.AddSubstep(substep)

	return substep
}

func (w *WorkflowStep) AddSubstep(substep *WorkflowStep) {
	if len(w.substeps) > 0 {
		w.substeps[len(w.substeps)-1].status = StatusSucceeded // If we go to the next substep, we're successful
	}
	w.substeps = append(w.substeps, substep)

	w.Notify()
}

func (w *WorkflowStep) SucceedWorkflow() {
	if w.status != StatusFailed {
		w.status = StatusSucceeded
	}
	for _, substep := range w.substeps {
		substep.SucceedWorkflow()
	}
	if w.nextStep != nil {
		w.nextStep.SucceedWorkflow()
	}

	w.Notify()
}

func (w *WorkflowStep) FailWorkflow() {
	if w.status != StatusSucceeded {
		w.status = StatusFailed
	}
	for _, substep := range w.substeps {
		substep.FailWorkflow()
	}
	if w.nextStep != nil {
		w.nextStep.FailWorkflow()
	}

	w.Notify()
}

func (w *WorkflowStep) Notify() {
	if w.updates != nil {
		w.updates <- MsgUpdated
	}
}

func (w *WorkflowStep) PrettyString() string {
	return w.toString(0, 0)
}

func (w *WorkflowStep) toString(parentIndent, indent int) string {
	builder := &strings.Builder{}

	indentString := ""
	if indent > 0 {
		terminator := "└─"
		if indent == parentIndent {
			terminator = "  "
		}
		indentString = strings.Repeat("  ", indent-1) + terminator
	}

	s := fmt.Sprintf("%s%s", indentString, w.name)

	style := styles.Info
	switch w.status {
	case StatusFailed:
		style = styles.Error
	case StatusRunning:
		style = styles.Info
	case StatusSucceeded:
		style = styles.Success
	}

	statusStyle := style.Copy().Bold(false).Italic(true)

	builder.WriteString(style.Render(s))
	builder.WriteString(statusStyle.Render(" -", string(w.status)))

	for _, child := range w.substeps {
		builder.WriteString("\n")
		builder.WriteString(child.toString(indent, indent+1))
	}

	if w.nextStep != nil {
		builder.WriteString("\n")
		builder.WriteString(w.nextStep.toString(indent, indent))
	}

	return builder.String()
}
