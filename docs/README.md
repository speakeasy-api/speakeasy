# speakeasy  
`speakeasy`  


The speakeasy cli tool provides access to the speakeasyapi.dev toolchain  

## Details

 A cli tool for interacting with the Speakeasy https://www.speakeasyapi.dev/ platform and its various functions including:
	- Generating Client SDKs from OpenAPI specs (go, python, typescript, java, php + more coming soon)
	- Validating OpenAPI specs
	- Interacting with the Speakeasy API to create and manage your API workspaces
	- Generating OpenAPI specs from your API traffic 								(coming soon)
	- Generating Postman collections from OpenAPI Specs 							(coming soon)


## Usage

```
speakeasy [flags]
```

### Options

```
  -h, --help   help for speakeasy
```

### Sub Commands

* [speakeasy api](api/README.md)	 - Access the Speakeasy API via the CLI
* [speakeasy auth](auth/README.md)	 - Authenticate the CLI
* [speakeasy docs](docs/README.md)	 - Use this command to generate content, compile, and publish SDK docs.
* [speakeasy generate](generate/README.md)	 - Generate Client SDKs, OpenAPI specs from request logs (coming soon) and more
* [speakeasy merge](merge.md)	 - Merge multiple OpenAPI documents into a single document
* [speakeasy overlay](overlay/README.md)	 - Work with OpenAPI Overlays
* [speakeasy proxy](proxy.md)	 - Proxy provides a reverse-proxy for debugging and testing Speakeasy's Traffic Capture capabilities
* [speakeasy suggest](suggest.md)	 - Validate an OpenAPI document and get fixes suggested by ChatGPT
* [speakeasy update](update.md)	 - Update the Speakeasy CLI to the latest version
* [speakeasy validate](validate/README.md)	 - Validate OpenAPI documents + more (coming soon)
