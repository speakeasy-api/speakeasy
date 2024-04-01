package bundler

import (
	"bytes"
	"context"
	"fmt"
	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	"os"
	"testing"
)

func Test(t *testing.T) {
	type args struct {
		name   string
		source workflow.Source
		want   string
	}

	dir, err := os.ReadDir("./testData")
	assert.NoError(t, err)

	var tests []args

	// Read the testData directory and create a test for each subdirectory
	for _, testDir := range dir {
		if !testDir.IsDir() {
			continue
		}

		test := args{
			name: testDir.Name(),
		}

		subdir, err := os.ReadDir(fmt.Sprintf("testData/%s", testDir.Name()))
		assert.NoError(t, err)

		for _, testDataFile := range subdir {
			if testDataFile.IsDir() {
				continue
			}

			if testDataFile.Name() == "want.yaml" {
				test.want = fmt.Sprintf("testData/%s/want.yaml", testDir.Name())
			} else if testDataFile.Name() == "workflow.yaml" {
				workflowFile, err := os.ReadFile(fmt.Sprintf("testData/%s/workflow.yaml", testDir.Name()))
				assert.NoError(t, err)

				var wf workflow.Workflow
				err = yaml.Unmarshal(workflowFile, &wf)
				assert.NoError(t, err)

				if len(wf.Sources) != 1 {
					assert.Fail(t, "expected exactly one source per workflow.yaml")
				}

				for _, source := range wf.Sources {
					test.source = source
				}
			}
		}

		tests = append(tests, test)
	}

	assert.NotEmpty(t, tests, "no tests found. Make sure you are running the test from the correct directory.")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer

			err := CompileSourceTo(context.Background(), nil, "", tt.source, &out)

			if tt.want == "" {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			want, err := os.ReadFile(tt.want)
			assert.NoError(t, err)

			doc1, err := libopenapi.NewDocumentWithConfiguration(want, &datamodel.DocumentConfiguration{
				AllowFileReferences:                 true,
				IgnorePolymorphicCircularReferences: true,
				IgnoreArrayCircularReferences:       true,
			})
			assert.NoError(t, err)

			doc2, err := libopenapi.NewDocumentWithConfiguration(out.Bytes(), &datamodel.DocumentConfiguration{
				AllowFileReferences:                 true,
				IgnorePolymorphicCircularReferences: true,
				IgnoreArrayCircularReferences:       true,
			})
			assert.NoError(t, err)

			documentChanges, errs := libopenapi.CompareDocuments(doc1, doc2)
			assert.Len(t, errs, 0)
			// When no changes, CompareDocuments returns nil
			assert.Nil(t, documentChanges)
		})
	}
}
