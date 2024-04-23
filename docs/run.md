# run  
`speakeasy run`  


generate an SDK, compile OpenAPI sources, and much more from a workflow.yaml file  

## Details

run the workflow(s) defined in your `.speakeasy/workflow.yaml` file.
A workflow can consist of multiple targets that define a source OpenAPI document that can be downloaded from a URL, exist as a local file, or be created via merging multiple OpenAPI documents together and/or overlaying them with an OpenAPI overlay document.
A full workflow is capable of running the following steps:
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
  -h, --help                      help for run
  -i, --installationURL string    the language specific installation URL for installation instructions if the SDK is not published to a package manager
      --installationURLs string   a map from target ID to installation URL for installation instructions if the SDK is not published to a package manager (default "null")
  -o, --output string             What to output while running (one of: summary, mermaid, console, default: summary) (default "summary")
  -r, --repo string               the repository URL for the SDK, if the published (-p) flag isn't used this will be used to generate installation instructions
  -b, --repo-subdir string        the subdirectory of the repository where the SDK is located in the repo, helps with documentation generation
      --repo-subdirs string       a map from target ID to the subdirectory of the repository where the SDK is located in the repo, helps with documentation generation (default "null")
      --skip-compile              skip compilation when generating the SDK
  -s, --source string             source to run. specify 'all' to run all sources
  -t, --target string             target to run. specify 'all' to run all targets
```

### Options inherited from parent commands

```
      --logLevel string   the log level (available options: [info, warn, error]) (default "info")
```

### Parent Command

* [speakeasy](README.md)	 - The speakeasy cli tool provides access to the speakeasyapi.dev toolchain
