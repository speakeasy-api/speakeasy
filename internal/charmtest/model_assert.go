package charmtest

import (
	"bytes"
	"testing"

	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/assert"
)

// Asserts that all expectation strings are contained in the model's output.
// Only use after the model output has been refreshed or updated.
func (m Model) AssertContains(t *testing.T, expectations ...string) {
	t.Helper()

	condition := func(output []byte) bool {
		for _, expectation := range expectations {
			if !bytes.Contains(output, []byte(expectation)) {
				return false
			}
		}
		return true
	}

	teatest.WaitFor(t, m.TestModel.Output(), condition)
}

// Asserts that a submitted form string field in the form exactly matches the
// expected value. Only use after the field has been submitted.
func (m Model) AssertFormStringEqual(t *testing.T, field string, expected string) {
	t.Helper()

	assert.Equal(t, expected, m.Form.GetString(field))
}
