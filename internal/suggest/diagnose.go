package suggest

import (
	"context"
	"github.com/gertd/go-pluralize"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/speakeasy-api/speakeasy/internal/schema"
	"strings"
)

type Diagnosis struct {
	InconsistentCasing   []string // list of operationIDs
	MissingTags          []string // list of operationIDs
	DuplicateInformation []string // list of operationIDs
	InconsistentTags     []string // list of groupIDs (tags)
}

type cases struct {
	camel, kebab, snake, mixed []string
}
type diagnoser struct {
	pluralizer *pluralize.Client

	operationCases cases
	tagsCases      cases
}

func Diagnose(ctx context.Context, schemaPath string) (*Diagnosis, error) {
	d := diagnoser{
		pluralizer: pluralize.NewClient(),
	}
	return d.diagnose(ctx, schemaPath)
}

func (d diagnoser) diagnose(ctx context.Context, schemaPath string) (*Diagnosis, error) {
	_, _, doc, err := schema.LoadDocument(ctx, schemaPath)
	if err != nil {
		return nil, err
	}

	diagnosis := &Diagnosis{}

	pathItems := doc.Model.Paths.PathItems

	for pathPair := orderedmap.First(pathItems); pathPair != nil; pathPair = pathPair.Next() {
		operations := pathPair.Value().GetOperations()

		for operationPair := orderedmap.First(operations); operationPair != nil; operationPair = operationPair.Next() {
			operation := operationPair.Value()
			operationID := operation.OperationId

			d.operationCases.detect(operationID)

			if len(operation.Tags) == 0 {
				diagnosis.MissingTags = append(diagnosis.MissingTags, operationID)
			}
			for _, tag := range operation.Tags {
				d.tagsCases.detect(tag)

				if d.containsDuplicateInformation(tag, operationID) {
					diagnosis.DuplicateInformation = append(diagnosis.DuplicateInformation, operationID)
				}
			}
		}
	}

	diagnosis.InconsistentCasing = d.operationCases.getInconsistent()
	diagnosis.InconsistentTags = d.tagsCases.getInconsistent()

	return diagnosis, nil
}

func (d diagnoser) containsDuplicateInformation(tag, operationID string) bool {
	tag = d.pluralizer.Singular(tag)
	return strings.Contains(strings.ToLower(operationID), strings.ToLower(tag))
}

func (c *cases) detect(s string) {
	isSnake, isKebab, isCamel := false, false, false
	count := 0

	if strings.Contains(s, "_") {
		isSnake = true
		count++
	}
	if strings.Contains(s, "-") {
		isKebab = true
		count++
	}
	if strings.ToLower(s) != s {
		isCamel = true
		count++
	}

	if count > 1 {
		c.mixed = append(c.mixed, s)
	} else if isSnake {
		c.snake = append(c.snake, s)
	} else if isKebab {
		c.kebab = append(c.kebab, s)
	} else if isCamel {
		c.camel = append(c.camel, s)
	}
}

// Figure out which casing style is most common. Add the operationIDs of the less common casing styles to the inconsistentCasing list
func (c *cases) getInconsistent() []string {
	res := c.mixed // All c.mixed case operationIDs are inconsistent

	if len(c.camel) > len(c.kebab) && len(c.camel) > len(c.snake) {
		res = append(res, c.kebab...)
		res = append(res, c.snake...)
	} else if len(c.kebab) > len(c.camel) && len(c.kebab) > len(c.snake) {
		res = append(res, c.camel...)
		res = append(res, c.snake...)
	} else if len(c.snake) > len(c.camel) && len(c.snake) > len(c.kebab) {
		res = append(res, c.camel...)
		res = append(res, c.kebab...)
	}

	return res
}
