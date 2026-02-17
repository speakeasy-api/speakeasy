package registry

import (
	"github.com/speakeasy-api/speakeasy/internal/ci/environment"
	"strings"
)

func ProcessRegistryTags() []string {
	var tags []string
	tagsInput := environment.RegistryTags()
	if len(strings.Replace(tagsInput, " ", "", -1)) == 0 {
		return tags
	}

	var processedTags []string
	if strings.Contains(tagsInput, "\n") {
		processedTags = strings.Split(tagsInput, "\n")
	} else {
		processedTags = strings.Split(tagsInput, ",")
	}

	for _, tag := range processedTags {
		tag = strings.Replace(tag, " ", "", -1)
		if len(tag) > 0 {
			tags = append(tags, tag)
		}
	}

	return tags
}
