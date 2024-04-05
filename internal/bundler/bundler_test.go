package bundler

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/AlekSi/pointer"
	"github.com/charmbracelet/log"
	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-core/bundler"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test(t *testing.T) {
	type args struct {
		name      string
		source    workflow.Source
		hasRemote bool
		wantErr   bool
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
			name:    testDir.Name(),
			wantErr: strings.HasSuffix(testDir.Name(), "_error"),
		}

		testDirPath := fmt.Sprintf("testData/%s", testDir.Name())
		subdir, err := os.ReadDir(testDirPath)
		assert.NoError(t, err)

		for _, testDataFile := range subdir {
			if testDataFile.IsDir() {
				if testDataFile.Name() == "remote" {
					test.hasRemote = true
				}
				continue
			}

			if testDataFile.Name() == "workflow.yaml" {
				workflowFile, err := os.ReadFile(filepath.Join(testDirPath, "workflow.yaml"))
				assert.NoError(t, err)

				var wf workflow.Workflow
				err = yaml.Unmarshal(workflowFile, &wf)
				assert.NoError(t, err)

				if len(wf.Sources) != 1 {
					assert.Fail(t, "expected exactly one source per workflow.yaml")
				}

				for _, source := range wf.Sources {
					source.Output = pointer.ToString(filepath.Join(testDirPath, "output/openapi.yaml"))

					for i, doc := range source.Inputs {
						if strings.HasPrefix(doc.Location, "./") {
							doc.Location = filepath.Join(testDirPath, doc.Location)
						}
						source.Inputs[i] = doc
					}
					for i, doc := range source.Overlays {
						if strings.HasPrefix(doc.Location, "./") {
							doc.Location = filepath.Join(testDirPath, doc.Location)
						}
						source.Overlays[i] = doc
					}

					test.source = source
				}

				j, _ := json.MarshalIndent(test.source, "", "  ")
				t.Errorf("workflow: %s", j)
			}
		}

		tests = append(tests, test)
	}

	assert.NotEmpty(t, tests, "no tests found. Make sure you are running the test from the correct directory.")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			if tt.hasRemote {
				logger := slog.New(log.New(os.Stderr))

				remoteFS := os.DirFS(fmt.Sprintf("./testData/%s/remote", tt.name))
				_, err := remoteFS.Open("components.yaml")
				assert.NoError(t, err)
				closeMockSrv, err := bundler.ServeFS(ctx, logger, ":8081", remoteFS)
				assert.NoError(t, err)
				defer closeMockSrv()
			}

			os.RemoveAll(filepath.Dir(*tt.source.Output))
			outputPath, err := CompileSource(context.Background(), nil, "", tt.source)

			if tt.wantErr {
				assert.Error(t, err)
				println(err.Error())
				return
			} else {
				assert.NoError(t, err)
			}

			have, err := os.ReadFile(outputPath)
			assert.NoError(t, err)

			// Comment this out if you want to inspect the output
			os.RemoveAll(filepath.Dir(*tt.source.Output))

			_, err = libopenapi.NewDocumentWithConfiguration(have, &datamodel.DocumentConfiguration{
				AllowRemoteReferences:               true,
				IgnorePolymorphicCircularReferences: true,
				IgnoreArrayCircularReferences:       true,
			})
			assert.NoError(t, err)
		})
	}
}
