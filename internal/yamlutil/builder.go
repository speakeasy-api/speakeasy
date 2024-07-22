package yamlutil

import (
	"gopkg.in/yaml.v3"
)

type Builder struct {
	isJSON bool
}

func NewBuilder(isJSON bool) *Builder {
	return &Builder{isJSON: isJSON}
}

func (b *Builder) NewNode(key string, value *yaml.Node) *yaml.Node {
	return &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{
				Kind:  yaml.ScalarNode,
				Value: key,
				Style: b.style(),
			},
			value,
		},
	}
}

func (b *Builder) NewListNode(key string, content []*yaml.Node) *yaml.Node {
	return &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{
				Kind:  yaml.ScalarNode,
				Value: key,
				Style: b.style(),
			},
			{
				Kind:    yaml.SequenceNode,
				Content: content,
			},
		},
	}
}

func (b *Builder) NewMultinode(keyVals ...string) *yaml.Node {
	var content []*yaml.Node

	for i := 0; i < len(keyVals); i += 2 {
		key := keyVals[i]
		value := keyVals[i+1]
		content = append(content, b.NewNodeItem(key, value)...)
	}

	return &yaml.Node{
		Kind:    yaml.MappingNode,
		Content: content,
	}
}

func (b *Builder) NewNodeItem(key, value string) []*yaml.Node {
	return []*yaml.Node{
		{Kind: yaml.ScalarNode, Value: key, Style: b.style()},
		{Kind: yaml.ScalarNode, Value: value, Style: b.style()},
	}
}

func (b *Builder) style() yaml.Style {
	if b.isJSON {
		return yaml.DoubleQuotedStyle
	}

	return 0
}
