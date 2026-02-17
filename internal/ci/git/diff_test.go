package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsGitDiffSignificant(t *testing.T) {
	tests := []struct {
		name           string
		diff           string
		ignorePatterns map[string]string
		want           bool
	}{
		{
			name: "gen.yaml changes are never significant",
			diff: `
diff --git a/gen.yaml b/gen.yaml
index 73093ef..000ec5e 100644
--- a/gen.yaml
+++ b/gen.yaml
@@ -4,6 +4,7 @@ management:
   docVersion: 0.1.0
~
   speakeasyVersion: 1.120.4
~
   generationVersion: 2.192.3
~
+#  I don't matter.
~
 generation:
~
   comments: {}
~
   sdkClassName: examplePackage
~
`,
			want: false,
		},
		{
			name: "code changes matter ; and when combined with gen.yaml changes, they are significant",
			diff: `
diff --git a/gen.yaml b/gen.yaml
index 73093ef..000ec5e 100644
--- a/gen.yaml
+++ b/gen.yaml
@@ -4,6 +4,7 @@ management:
   docVersion: 0.1.0
~
   speakeasyVersion: 1.120.4
~
   generationVersion: 2.192.3
~
+#  I don't matter.
~
 generation:
~
   comments: {}
~
   sdkClassName: examplePackage
~
diff --git a/internal/planmodifiers/boolplanmodifier/suppress_diff.go b/internal/planmodifiers/boolplanmodifier/suppress_diff.go
index 395e4b2..dcdc177 100644
--- a/internal/planmodifiers/boolplanmodifier/suppress_diff.go
+++ b/internal/planmodifiers/boolplanmodifier/suppress_diff.go
@@ -2,6 +2,9 @@
 
~
 package boolplanmodifier
~
 
~
+// code changes do matter
~
+// multiline too
~
~
 import (
~
 	"context"
~
 	"github.com/exampleAuthor/terraform-provider-examplePackage/internal/planmodifiers/utils"
~
`,
			ignorePatterns: map[string]string{
				"1.120.4": "1.120.5",
			},
			want: true,
		},
		{
			name: "ignores a version number change, even when compiled into an unusual line",
			diff: `diff --git a/README.md b/README.md
index 3db161d..f1ae144 100755
--- a/README.md
+++ b/README.md
@@ -10,7 +10,7 @@ terraform {
   required_providers {
~
     examplePackage = {
~
       source  = "exampleAuthor/examplePackage"
~
       version = 
-"0.13.5"
+"0.13.6"
~
     }
~
   }
~
 }
~
diff --git a/RELEASES.md b/RELEASES.md
index e30e211..84bc6ed 100644
--- a/RELEASES.md
+++ b/RELEASES.md
@@ -206,4 +206,12 @@ Based on:
 - OpenAPI Doc 0.1.0 
~
 - Speakeasy CLI 1.120.3 (2.192.1) https://github.com/speakeasy-api/speakeasy
~
 ### Generated
~
 - [terraform v0.13.5] .
~
~
+## 2023-11-17 00:37:46
~
+### Changes
~
+Based on:
~
+- OpenAPI Doc 0.1.0 
~
+- Speakeasy CLI 1.120.4 (2.192.3) https://github.com/speakeasy-api/speakeasy
~
+### Generated
~
+- [terraform v0.13.6] .
~
diff --git a/docs/index.md b/docs/index.md
index d6b3918..931847a 100644
--- a/docs/index.md
+++ b/docs/index.md
@@ -17,7 +17,7 @@ terraform {
   required_providers {
~
     examplePackage = {
~
       source  = "exampleAuthor/examplePackage"
~
       version = 
-"0.13.5"
+"0.13.6"
~
     }
~
   }
~
 }
~
diff --git a/examples/provider/provider.tf b/examples/provider/provider.tf
index 92f99fe..b32e0fd 100644
--- a/examples/provider/provider.tf
+++ b/examples/provider/provider.tf
@@ -2,7 +2,7 @@ terraform {
   required_providers {
~
     examplePackage = {
~
       source  = "exampleAuthor/examplePackage"
~
       version = 
-"0.13.5"
+"0.13.6"
~
     }
~
   }
~
 }
~
diff --git a/gen.yaml b/gen.yaml
index de293be..73093ef 100644
--- a/gen.yaml
+++ b/gen.yaml
@@ -1,9 +1,9 @@
 configVersion: 1.0.0
~
 management:
~
   docChecksum: 
-123e1a1abd36f630cf6d5432e0649e38
+c0a14e297e308a50b297260c06a1bfe0
~
   docVersion: 0.1.0
~
   speakeasyVersion: 
-1.120.3
+1.120.4
~
   generationVersion: 
-2.192.1
+2.192.3
~
 generation:
~
   comments: {}
~
   sdkClassName: examplePackage
~
@@ -21,7 +21,7 @@ features:
     nameOverrides: 2.81.1
~
     unions: 2.81.5
~
 terraform:
~
   version: 
-0.13.5
+0.13.6
~
   author: exampleAuthor
~
   imports:
~
     option: openapi
~
diff --git a/internal/sdk/sdk.go b/internal/sdk/sdk.go
index c093fc3..67a2a32 100644
--- a/internal/sdk/sdk.go
+++ b/internal/sdk/sdk.go
@@ -151,9 +151,9 @@ func New(opts ...SDKOption) *examplePackage {
 		sdkConfiguration: sdkConfiguration{
~
 			Language:          "go",
~
 			OpenAPIDocVersion: "0.1.0",
~
 			SDKVersion:        
-"0.13.5",
+"0.13.6",
~
 			GenVersion:        
-"2.192.1",
+"2.192.3",
~
 			UserAgent:         "speakeasy-sdk/go 
-0.13.5 2.192.1
+0.13.6 2.192.3
  0.1.0 examplePackage",
~
 		},
~
 	}
~
 	for _, opt := range opts {
~
`,
			ignorePatterns: map[string]string{
				"0.13.5":  "0.13.6",
				"2.192.1": "2.192.3",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := IsGitDiffSignificant(tt.diff, tt.ignorePatterns)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
