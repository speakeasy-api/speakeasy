package ci

import "os"

// setEnvIfNotEmpty sets an environment variable only if the provided value is non-empty.
// This is used to bridge CLI flags to environment variables. When a flag has a value
// (either from CLI args or from its DefaultValue which reads the env var), it ensures
// the env var is set for code that reads it directly.
//
// This enables backward compatibility: action code can continue to read os.Getenv()
// while the CLI provides a flag-based interface on top.
func setEnvIfNotEmpty(key, value string) {
	if value != "" {
		_ = os.Setenv(key, value)
	}
}

// setEnvBool sets an environment variable to "true" or "false" based on the bool value.
func setEnvBool(key string, value bool) {
	if value {
		_ = os.Setenv(key, "true")
	} else {
		_ = os.Setenv(key, "false")
	}
}
