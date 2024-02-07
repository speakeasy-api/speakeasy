package run

import (
	"fmt"
	"strings"
)

const classDefs = `
classDef error stroke:#CE262A,color:#CE262A
classDef success stroke:#63AC67,color:#63AC67
classDef running stroke:#293D53,color:#293D53
classDef skipped stroke:#D3D3D3,color:#D3D3D3
`

func (w *WorkflowStep) ToMermaidDiagram() string {
	builder := strings.Builder{}
	builder.WriteString("```mermaid\n")
	builder.WriteString("flowchart LR\n")

	i := 0

	// Iterate down a level because mermaid only supports two levels of nesting for subgraphs
	for _, substep := range w.substeps {
		builder.WriteString(substep.toMermaidInternal(&i, 0))
	}

	builder.WriteString(classDefs)
	builder.WriteString("```\n")

	return builder.String()
}

func (w *WorkflowStep) toMermaidInternal(nodeNum *int, depth int) string {
	builder := strings.Builder{}

	if depth > 2 {
		panic("mermaid only supports two levels of nesting for subgraphs")
	}

	writeChildNode := func(child *WorkflowStep) {
		builder.WriteString(child.toMermaidInternal(nodeNum, depth+1))
	}
	writeConnection := func(from, to int) {
		builder.WriteString(fmt.Sprintf("%d --> %d\n", from, to))
	}

	*nodeNum++
	selfNodeNum := *nodeNum

	var class, statusMessage string
	switch w.status {
	case StatusFailed:
		class = "error"
		statusMessage = " - failed"
	case StatusRunning:
		class = "running"
	case StatusSucceeded:
		class = "success"
	case StatusSkipped:
		class = "skipped"
		statusMessage = " - skipped"
	}

	if statusMessage != "" && w.statusExplanation != "" {
		statusMessage = fmt.Sprintf("%s (%s)", statusMessage, w.statusExplanation)
	}

	nodeNameDisplay := fmt.Sprintf("%s%s", w.name, statusMessage)

	if len(w.substeps) == 0 {
		builder.WriteString(fmt.Sprintf("%d(\"%s\"):::%s\n", selfNodeNum, nodeNameDisplay, class))
	} else {
		builder.WriteString(fmt.Sprintf("subgraph %d [\"%s\"]\n", selfNodeNum, nodeNameDisplay))
		for i, child := range w.substeps {
			childNodeNum := *nodeNum + 1
			writeChildNode(child)

			if i < len(w.substeps)-1 {
				writeConnection(childNodeNum, *nodeNum+1)
			}
		}
		builder.WriteString("end\n")
		builder.WriteString(fmt.Sprintf("class %d %s\n", selfNodeNum, class)) // Subgraph class assignment needs to happen after the subgraph is defined
	}

	return builder.String()
}
