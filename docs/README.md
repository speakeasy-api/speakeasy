# speakeasy  
`speakeasy`  


The Speakeasy CLI tool provides access to the Speakeasy.com platform  

## Details

# Speakeasy 

A CLI tool for interacting with the [Speakeasy platform](https://www.speakeasy.com/) and its APIs.

Use this CLI to:
- Lint and validate OpenAPI specs
- Create, manage, and run Speakeasy workflows
- Configure GitHub Actions for Speakeasy workflows
- Suggest improvements to OpenAPI specs

Generate from OpenAPI Specs:
- Client and Server SDKs in GO, Python, TypeScript, Java, PHP, C#, Swift, Ruby
- Postman collections
- Terraform providers

[Quickstart guide](https://www.speakeasy.com/docs/create-client-sdks)

Visit [Speakeasy](https://www.speakeasy.com/) for more information


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
* [speakeasy run](run.md)	 - Run all the workflows defined in your workflow.yaml file. This can include multiple SDK generations from different OpenAPI sources
* [speakeasy status](status.md)	 - Review status of current workspace
* [speakeasy suggest](suggest/README.md)	 - Automatically improve your OpenAPI document with an LLM
* [speakeasy tag](tag/README.md)	 - Add tags to a given revision of your API. Specific to a registry namespace
* [speakeasy update](update.md)	 - Update the Speakeasy CLI to the latest version
