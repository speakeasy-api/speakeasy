# run  
`speakeasy run`  


Run all the workflows defined in your workflow.yaml file. This can include multiple SDK generations from different OpenAPI sources  

## Details

# Run 
 Execute the workflow(s) defined in your `.speakeasy/workflow.yaml` file.

A workflow can consist of multiple targets that define a source OpenAPI document that can be downloaded from a URL, exist as a local file, or be created via merging multiple OpenAPI documents together and/or overlaying them with an OpenAPI overlay document.

A full workflow is capable of running the following:
  - Downloading source OpenAPI documents from a URL
  - Merging multiple OpenAPI documents together
  - Overlaying OpenAPI documents with an OpenAPI overlay document
  - Generating one or many SDKs from the resulting OpenAPI document
  - Compiling the generated SDKs

If `speakeasy run` is run without any arguments it will run either the first target in the workflow or the first source in the workflow if there are no other targets or sources, otherwise it will prompt you to select a target or source to run.

## Usage

```
speakeasy run [flags]
```

### Options

```
  -d, --debug                     enable writing debug files with broken code
      --force                     Force generation of SDKs even when no changes are present
      --github                    kick off a generation run in GitHub
  -h, --help                      help for run
  -i, --installationURL string    the language specific installation URL for installation instructions if the SDK is not published to a package manager
      --installationURLs string   a map from target ID to installation URL for installation instructions if the SDK is not published to a package manager (default "null")
      --launch-studio             launch the web studio for improving the quality of the generated SDK
  -o, --output string             What to output while running (default "summary")
      --registry-tags strings     tags to apply to the speakeasy registry bundle (comma-separated list)
  -r, --repo string               the repository URL for the SDK, if the published (-p) flag isn't used this will be used to generate installation instructions
  -b, --repo-subdir string        the subdirectory of the repository where the SDK is located in the repo, helps with documentation generation
      --repo-subdirs string       a map from target ID to the subdirectory of the repository where the SDK is located in the repo, helps with documentation generation (default "null")
      --set-version string        the manual version to apply to the generated SDK
      --skip-compile              skip compilation when generating the SDK
      --skip-versioning           skip automatic SDK version increments
  -s, --source string             source to run. specify 'all' to run all sources
  -t, --target string             target to run. specify 'all' to run all targets
      --verbose                   Verbose logging
```

### Options inherited from parent commands

```
      --logLevel string   the log level (available options: [info, warn, error]) (default "info")
```

### Parent Command

* [speakeasy](README.md)	 - The Speakeasy CLI tool provides access to the Speakeasy.com platform
