package prompts

import (
	"strings"

	"github.com/iancoleman/strcase"
)

// Converts a Go module path to a desirable Go package name. For example, if the
// module path is "github.com/owner/repo-go-sdk", the package name will be
// "repo".
func goModulePathToPackageName(modulePath string) string {
	var sdkPackageName string

	if strings.Contains(modulePath, "/") {
		// If the module path contains a slash, we assume it's a
		// full module path and use the last segment as the package
		// name.
		parts := strings.Split(modulePath, "/")
		sdkPackageName = parts[len(parts)-1]
	} else {
		sdkPackageName = modulePath
	}

	// Automatically clean up the final segment to be more desirable package name
	sdkPackageName = strings.TrimSuffix(sdkPackageName, "-go")
	sdkPackageName = strings.TrimSuffix(sdkPackageName, "-go-sdk")
	sdkPackageName = strings.TrimSuffix(sdkPackageName, "-sdk")

	sdkPackageName = strings.ToLower(strcase.ToCamel(sdkPackageName))

	return sdkPackageName
}
