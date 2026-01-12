package workflowTracking

import (
	"fmt"
	"strings"

	"github.com/speakeasy-api/huh"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/log"
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

	for _, substep := range w.substeps {
		substep.Succeed()
	}
}

func (w *WorkflowStep) SucceedWorkflow() {
	w.Succeed()
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
	var msg StatusMsg
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

// RunPrompt sends a prompt form to the visualizer and blocks until completion.
// The form's bound values will be populated when this returns.
// Returns nil on success, huh.ErrUserAborted if user cancelled, or other error.
// If the updates channel is nil (no visualizer), runs the form directly.
func (w *WorkflowStep) RunPrompt(form *huh.Form) error {
	if w == nil || w.updates == nil {
		// No visualizer - run form directly
		return form.Run()
	}

	respCh := make(chan error, 1)
	w.updates <- PromptRequestMsg{
		Form:   form,
		RespCh: respCh,
	}

	// Block until the visualizer completes the form
	return <-respCh
}

func (w *WorkflowStep) PrettyString() string {
	return w.toString(0, 0)
}

// ListenForSubsteps listens for progress messages and creates/updates workflow steps.
// It handles both legacy MsgGithub/MsgStudio messages and new MsgStep messages with path-based hierarchy.
func (w *WorkflowStep) ListenForSubsteps(c chan log.Msg) {
	// Track steps by ID for status updates
	stepsByID := make(map[string]*WorkflowStep)

	for msg := range c {
		switch msg.Type {
		case log.MsgGithub:
			// Legacy: handle ::group:: messages
			if strings.HasPrefix(msg.Msg, "::group::") {
				stepName := strings.TrimSpace(strings.TrimPrefix(msg.Msg, "::group::"))
				w.NewSubstep(stepName)
			}

		case log.MsgStudio:
			// Legacy: handle studio messages
			w.NewSubstep(msg.Msg)

		case log.MsgStepSkipped:
			// Legacy: handle skipped step messages
			stepName := strings.TrimSpace(strings.TrimPrefix(msg.Msg, "::group::"))
			skippedStep := w.findOrCreateSubstep(stepName)
			skippedStep.Skip("skipped")

		case log.MsgStep:
			// New: handle path-based step messages with status updates
			if msg.Step == nil || len(msg.Step.Path) == 0 {
				continue
			}

			// Check if this is an update to an existing step
			if existing, ok := stepsByID[msg.Step.ID]; ok {
				existing.setStatus(msg.Step.Status)
				continue
			}

			// Create/find the hierarchy and register the leaf step
			parent := w
			for i, name := range msg.Step.Path {
				substep := parent.findOrCreateSubstep(name)
				if i == len(msg.Step.Path)-1 {
					// This is the leaf node - register it and set status
					stepsByID[msg.Step.ID] = substep
					substep.setStatus(msg.Step.Status)
				}
				parent = substep
			}
		}
	}
}

// findOrCreateSubstep finds an existing substep by name or creates a new one.
func (w *WorkflowStep) findOrCreateSubstep(name string) *WorkflowStep {
	for _, s := range w.substeps {
		if s.name == name {
			return s
		}
	}
	return w.NewSubstep(name)
}

// setStatus updates the step's status based on a StepStatus value.
func (w *WorkflowStep) setStatus(status log.StepStatus) {
	switch status {
	case log.StepStatusPending:
		w.status = StatusRunning
	case log.StepStatusSuccess:
		w.status = StatusSucceeded
	case log.StepStatusFailed:
		w.status = StatusFailed
	case log.StepStatusSkipped:
		w.status = StatusSkipped
	}
	w.notify()
}

// Example output:
//   - success: A -> B -> C
//   - failure: A -> B -> C
func (w *WorkflowStep) LastStepToString() string {
	step := w
	var status Status
	var stepNames = []string{}

	for {
		stepNames = append(stepNames, step.name)
		status = step.status

		if len(step.substeps) == 0 {
			break
		}
		step = step.substeps[len(step.substeps)-1]
	}

	return fmt.Sprintf("%s: %s", status, strings.Join(stepNames, " -> "))
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

	style := styles.Info.Bold(true)
	switch w.status {
	case StatusFailed:
		style = styles.Error.Bold(true)
	case StatusRunning:
		style = styles.Info.Bold(true)
	case StatusSucceeded:
		style = styles.Success.Bold(true)
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
