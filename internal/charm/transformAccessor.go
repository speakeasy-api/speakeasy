package charm

import (
	"github.com/charmbracelet/huh"
)

type TransformAccessor struct {
	value     *string
	transform func(string) string
}

func (t *TransformAccessor) Get() string {
	if t.value == nil {
		return ""
	}
	return *t.value
}

func (t *TransformAccessor) Set(value string) {
	value = t.transform(value)
	*t.value = value
}

var _ huh.Accessor[string] = &TransformAccessor{}
