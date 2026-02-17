package utils

import (
	"strings"
)

// Returns the directory output name for the given target name. This
// automatically handles when the target name contains hyphens.
func OutputTargetDirectory(targetName string) string {
	targetName = strings.ReplaceAll(targetName, "-", "_")
	return targetName + "_directory"
}

// Returns the MCP release output name for the given target name. This
// automatically handles when the target name contains hyphens and trims any
// "mcp-" prefix.
func OutputTargetMCPRelease(targetName string) string {
	targetName = strings.TrimPrefix(targetName, "mcp-")
	targetName = strings.ReplaceAll(targetName, "-", "_")
	return "mcp_release_" + targetName
}

// Returns the publish output name for the given target name. This
// automatically handles when the target name contains hyphens.
func OutputTargetPublish(targetName string) string {
	targetName = strings.ReplaceAll(targetName, "-", "_")
	return "publish_" + targetName
}

// Returns the regenerated output name for the given target name. This
// automatically handles when the target name contains hyphens.
func OutputTargetRegenerated(targetName string) string {
	targetName = strings.ReplaceAll(targetName, "-", "_")
	return targetName + "_regenerated"
}
