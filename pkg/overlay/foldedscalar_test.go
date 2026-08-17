package overlay

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const foldedWithMoreIndentedLine = `description: >-
  ### Authorization

  | Type | Scopes |
  | ---- | ------ |
   | Organization | ` + "`manage_passwords`" + ` |
`

func roundTrip(t *testing.T, doc string, stabilize bool) string {
	t.Helper()

	var node yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(doc), &node))

	if stabilize {
		stabilizeFoldedScalars(&node)
	}

	out, err := yaml.Marshal(&node)
	require.NoError(t, err)

	return string(out)
}

func decodeDescription(t *testing.T, doc string) string {
	t.Helper()

	var decoded struct {
		Description string `yaml:"description"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(doc), &decoded))

	return decoded.Description
}

func TestStabilizeFoldedScalarsSurvivesRepeatedApplies(t *testing.T) {
	t.Parallel()

	want := decodeDescription(t, foldedWithMoreIndentedLine)

	doc := foldedWithMoreIndentedLine
	for i := range 30 {
		doc = roundTrip(t, doc, true)
		assert.Equal(t, want, decodeDescription(t, doc), "value changed after %d applies", i+1)
	}
}

// Fails once gopkg.in/yaml.v3 fixes the emitter, at which point stabilizeFoldedScalars can go.
func TestFoldedScalarGrowsWithoutStabilizer(t *testing.T) {
	t.Parallel()

	before := decodeDescription(t, foldedWithMoreIndentedLine)
	after := decodeDescription(t, roundTrip(t, foldedWithMoreIndentedLine, false))

	assert.NotEqual(t, before, after)
}

func TestStabilizeFoldedScalarsLeavesStableStylesAlone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		doc  string
	}{
		{
			name: "folded scalar with no more-indented line",
			doc:  "description: >-\n  one line\n  another line\n",
		},
		{
			name: "literal scalar with more-indented line",
			doc:  "description: |-\n  one line\n   more indented\n",
		},
		{
			name: "plain scalar",
			doc:  "description: just a string\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			want := decodeDescription(t, tt.doc)

			doc := tt.doc
			for range 5 {
				doc = roundTrip(t, doc, true)
				assert.Equal(t, want, decodeDescription(t, doc))
			}
		})
	}
}

func TestHasMoreIndentedLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "no indentation", value: "one\ntwo", want: false},
		{name: "space indented line", value: "one\n two", want: true},
		{name: "tab indented line", value: "one\n\ttwo", want: true},
		{name: "blank lines only", value: "one\n\ntwo", want: false},
		{name: "empty", value: "", want: false},
		{name: "first line indented", value: " one\ntwo", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, hasMoreIndentedLine(tt.value))
		})
	}
}
