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

* [speakeasy auth](auth/README.md)	 - Authenticate the CLI
* [speakeasy bump](bump.md)	 - Bumps the version of a Speakeasy Generation Target
* [speakeasy configure](configure/README.md)	 - Configure your Speakeasy SDK Setup.
* [speakeasy generate](generate/README.md)	 - Generate client SDKs, docsites, and more
* [speakeasy lint](lint/README.md)	 - Lint/Validate OpenAPI documents and Speakeasy configuration files
* [speakeasy merge](merge.md)	 - Merge multiple OpenAPI documents into a single document
* [speakeasy openapi](openapi/README.md)	 - Validate and compare OpenAPI documents
* [speakeasy overlay](overlay/README.md)	 - Work with OpenAPI Overlays
* [speakeasy quickstart](quickstart.md)	 - Guided setup to help you create a new SDK in minutes.
* [speakeasy run](run.md)	 - run the workflow defined in your workflow.yaml file
* [speakeasy suggest](suggest.md)	 - Validate an OpenAPI document and get fixes suggested by ChatGPT
* [speakeasy update](update.md)	 - Update the Speakeasy CLI to the latest version
