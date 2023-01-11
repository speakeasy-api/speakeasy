# speakeasy  
`speakeasy`  


The speakeasy cli tool provides access to the speakeasyapi.dev toolchain  

## Details

 A cli tool for interacting with the Speakeasy https://www.speakeasyapi.dev/ platform and its various functions including:
	- Generating Client SDKs from OpenAPI specs (go, python, typescript(web/server), + more coming soon)
	- Interacting with the Speakeasy API to create and manage your API workspaces	(coming soon)
	- Generating OpenAPI specs from your API traffic 								(coming soon)
	- Validating OpenAPI specs 														(coming soon)
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
* [speakeasy generate](generate/README.md)	 - Generate Client SDKs, OpenAPI specs from request logs (coming soon) and more
* [speakeasy validate](validate/README.md)	 - Validate OpenAPI documents + more (coming soon)
