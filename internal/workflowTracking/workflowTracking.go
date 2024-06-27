package workflowTracking

import (
	"fmt"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/log"

	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
)

type Status string

const (
	StatusRunning   Status = "running"
	StatusFailed    Status = "failed"
	StatusSucceeded Status = "success"
	StatusSkipped   Status = "skipped"
)

type WorkflowStep struct {
	name              string
	status            Status
	statusExplanation string
	substeps          []*WorkflowStep
	updates           chan<- UpdateMsg
	logger            log.Logger
}

func NewWorkflowStep(name string, logger log.Logger, updatesListener chan<- UpdateMsg) *WorkflowStep {
	return &WorkflowStep{
		name:     name,
		status:   StatusRunning,
		substeps: []*WorkflowStep{},
		updates:  updatesListener,
		logger:   logger,
	}
}

func (w *WorkflowStep) NewSubstep(name string) *WorkflowStep {
	if w == nil {
		return nil
	}

	substep := NewWorkflowStep(name, w.logger, w.updates)

	w.AddSubstep(substep)

	w.logger.PrintfStyled(styles.Dimmed, "\n» %s...\n", name)

	return substep
}

func (w *WorkflowStep) AddSubstep(substep *WorkflowStep) {
	if len(w.substeps) > 0 {
		prev := w.substeps[len(w.substeps)-1]
		prev.Succeed() // If we go to the next substep, we're successful
	}
	w.substeps = append(w.substeps, substep)

	w.notify()
}

func (w *WorkflowStep) Skip(reason string) {
	w.status = StatusSkipped
	w.statusExplanation = reason
	w.logger.Infof("\nStep skipped: %s (reason: %s)\n", w.name, reason)
	w.notify()
}

func (w *WorkflowStep) Succeed() {
	if w.status == StatusRunning {
		w.status = StatusSucceeded
	}
}

func (w *WorkflowStep) SucceedWorkflow() {
	if w.status == StatusRunning {
		w.status = StatusSucceeded
	}
	for _, substep := range w.substeps {
		substep.SucceedWorkflow()
	}

	w.notify()
}

func (w *WorkflowStep) FailWorkflow() {
	w.FailWorkflowWithoutNotifying()
	w.notify()
}

func (w *WorkflowStep) Fail() {
	w.failLastSubstep() // Otherwise the parent step might fail even though all the child steps say "success"

	if w.status == StatusRunning {
		w.status = StatusFailed
		w.logger.Errorf("\nStep Failed: %s\n", w.name)
	}
}

func (w *WorkflowStep) failLastSubstep() {
	for i, substep := range w.substeps {
		if len(substep.substeps) > 0 {
			substep.failLastSubstep()
		} else if i == len(w.substeps)-1 { // Fail the last substep only
			substep.Fail()
		}
	}
}

func (w *WorkflowStep) FailWorkflowWithoutNotifying() {
	w.Fail()
	for _, substep := range w.substeps {
		substep.FailWorkflowWithoutNotifying()
	}
}

func (w *WorkflowStep) Finalize(succeeded bool) {
	var msg UpdateMsg
	if succeeded {
		w.SucceedWorkflow()
		msg = MsgSucceeded
	} else {
		w.FailWorkflow()
		msg = MsgFailed
	}

	// We send the final messages here rather than in succeed/fail because those can be called on substeps.
	// Also they are recursive, and we only want to send the finalize messages once
	if w.updates != nil {
		w.updates <- msg
	}
}

func (w *WorkflowStep) notify() {
	if w.updates != nil {
		w.updates <- MsgUpdated
	}
}

func (w *WorkflowStep) PrettyString() string {
	return w.toString(0, 0)
}

func (w *WorkflowStep) ListenForSubsteps(c chan log.Msg) {
	msg := <-c
	if msg.Type == log.MsgGithub && strings.HasPrefix(msg.Msg, "::group::") {
		stepName := strings.TrimPrefix(msg.Msg, "::group::")
		stepName = strings.TrimSpace(stepName)
		w.NewSubstep(stepName)
	}
	w.ListenForSubsteps(c)
}

// Example output:
//   - success: A -> B -> C
//   - failure: A -> B -> C
func (w *WorkflowStep) LastStepToString() string {
	step := w
	var status Status = StatusSucceeded
	var stringNames = []string{}

	for {
		stringNames = append(stringNames, step.name)
		status = step.status

		if len(step.substeps) == 0 {
			break
		}
		step = step.substeps[len(step.substeps)-1]
	}

	return fmt.Sprintf("%s: %s", status, strings.Join(stringNames, " -> "))
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
	case StatusSkipped:
		style = styles.Dimmed
	}

	statusStyle := style.Bold(false).Italic(true)

	builder.WriteString(style.Render(s))
	builder.WriteString(statusStyle.Render(" -", string(w.status)))

	if w.statusExplanation != "" {
		builder.WriteString(statusStyle.Render(fmt.Sprintf(" (%s)", w.statusExplanation)))
	}

	for _, child := range w.substeps {
		builder.WriteString("\n")
		builder.WriteString(child.toString(indent, indent+1))
	}

	return builder.String()
}
