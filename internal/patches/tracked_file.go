package patches

import "github.com/speakeasy-api/sdk-gen-config/lockfile"

const (
	trackedFileMovedToKey    = "moved_to"
	trackedFileMovedToAltKey = "movedTo"
)

func GetMovedTo(tracked lockfile.TrackedFile) string {
	if tracked.AdditionalProperties == nil {
		return ""
	}

	for _, key := range []string{trackedFileMovedToKey, trackedFileMovedToAltKey} {
		if value, ok := tracked.AdditionalProperties[key].(string); ok {
			return value
		}
	}

	return ""
}

func SetMovedTo(tracked *lockfile.TrackedFile, path string) {
	if tracked.AdditionalProperties == nil {
		tracked.AdditionalProperties = map[string]any{}
	}

	delete(tracked.AdditionalProperties, trackedFileMovedToKey)
	delete(tracked.AdditionalProperties, trackedFileMovedToAltKey)

	if path != "" {
		tracked.AdditionalProperties[trackedFileMovedToKey] = path
	}

	if len(tracked.AdditionalProperties) == 0 {
		tracked.AdditionalProperties = nil
	}
}
