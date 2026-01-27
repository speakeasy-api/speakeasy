package github

import "regexp"

// ExecutionIDComment formats an execution ID as an HTML comment for PR descriptions.
func ExecutionIDComment(executionID string) string {
	return "<!-- execution_id: " + executionID + " -->"
}

// executionIDPattern matches the execution ID HTML comment format.
var executionIDPattern = regexp.MustCompile(`<!--\s*execution_id:\s*([^\s]+)\s*-->`)

// ParseExecutionIDComment extracts the execution ID from an HTML comment.
// Returns empty string if not found.
func ParseExecutionIDComment(text string) string {
	if matches := executionIDPattern.FindStringSubmatch(text); len(matches) > 1 {
		return matches[1]
	}
	return ""
}
