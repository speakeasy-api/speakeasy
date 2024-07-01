# speakeasy  
`speakeasy`  


The speakeasy cli tool provides access to the speakeasyapi.dev toolchain  

## Details

 A cli tool for interacting with the Speakeasy https://www.speakeasyapi.dev/ platform and its various functions including:
	- Generating Client SDKs from OpenAPI specs (go, python, typescript, java, php, c#, swift, ruby, terraform)
	- Validating OpenAPI specs
	- Interacting with the Speakeasy API to create and manage your API workspaces
	- Generating OpenAPI specs from your API traffic
	- Generating Postman collections from OpenAPI Specs


## Usage

```
speakeasy [flags]
```

### Options

```
  -h, --help              help for speakeasy
      --logLevel string   the log level (available options: [info, warn, error]) (default "info")
```

### Sub Commands

* [speakeasy ask](ask.md)	 - Starts a conversation with Speakeasy trained AI
* [speakeasy auth](auth/README.md)	 - Authenticate the CLI
* [speakeasy bump](bump.md)	 - Bumps the version of a Speakeasy Generation Target
* [speakeasy clean](clean.md)	 - Speakeasy clean can be used to clean up cache, stale temp folders, and old CLI binaries.
* [speakeasy configure](configure/README.md)	 - Configure your Speakeasy SDK Setup.
* [speakeasy generate](generate/README.md)	 - One off Generations for client SDKs and more
* [speakeasy lint](lint/README.md)	 - Lint/Validate OpenAPI documents and Speakeasy configuration files
* [speakeasy merge](merge.md)	 - Merge multiple OpenAPI documents into a single document
* [speakeasy openapi](openapi/README.md)	 - Utilities for working with OpenAPI documents
* [speakeasy overlay](overlay/README.md)	 - Work with OpenAPI Overlays
* [speakeasy quickstart](quickstart.md)	 - Guided setup to help you create a new SDK in minutes.
* [speakeasy run](run.md)	 - generate an SDK, compile OpenAPI sources, and much more from a workflow.yaml file
* [speakeasy suggest](suggest/README.md)	 - Automatically improve your OpenAPI document with an LLM
* [speakeasy tag](tag/README.md)	 - Add tags to a given revision of your API. Specific to a registry namespace
* [speakeasy update](update.md)	 - Update the Speakeasy CLI to the latest version
