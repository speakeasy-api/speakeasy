package integration_tests

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMemoryIssue(t *testing.T) {
	require.NoError(t, executeI(t, "/tmp/docusign", "generate", "sdk", "-s", "docusign-openapi-3.1-spec.2.json.yaml", "-l", "typescript", "-o", ".", "--debug").Run())
}
