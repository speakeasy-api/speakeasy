# sdk  
`speakeasy generate sdk`  


Generating Client SDKs from OpenAPI specs (go, python, typescript, java + more coming soon)  

## Details

Using the "speakeasy generate sdk" command you can generate client SDK packages for various languages
that are ready to use and publish to your favorite package registry.

The following languages are currently supported:
	- go
	- python
	- typescript
	- java
	- more coming soon

By default the command will generate a Go SDK, but you can specify a different language using the --lang flag.
It will also use generic defaults for things such as package name (openapi), etc.

# Configuration

To configure the package of the generated SDKs you can config a "gen.yaml" file in the root of the output directory.

Example gen.yaml file for Go SDK:

```
go:
  packagename: github.com/speakeasy-api/speakeasy-client-sdk-go
  version: 0.1.0
# baseserverurl optional, if not specified it will use the server URL from the OpenAPI spec 
# this can also be provided via the --baseurl flag when calling the command line
baseserverurl: https://api.speakeasyapi.dev 
```

Example gen.yaml file for Python SDK:

```
python:
  packagename: speakeasy-client-sdk-python
  version: 0.1.0
  description: Speakeasy API Client SDK for Python
  author: Speakeasy API
# baseserverurl optional, if not specified it will use the server URL from the OpenAPI spec 
# this can also be provided via the --baseurl flag when calling the command line
baseserverurl: https://api.speakeasyapi.dev 
```

Example gen.yaml file for Typescript SDK:

```
typescript:
  packagename: speakeasy-client-sdk-typescript
  version: 0.1.0
  author: Speakeasy API
# baseserverurl optional, if not specified it will use the server URL from the OpenAPI spec
# this can also be provided via the --baseurl flag when calling the command line
baseserverurl: https://api.speakeasyapi.dev
```

Example gen.yaml file for Java SDK:

```
java:
  packagename: dev.speakeasyapi.javasdk
  projectname: speakeasy-client-sdk-java
  version: 0.1.0
# baseserverurl optional, if not specified it will use the server URL from the OpenAPI spec
# this can also be provided via the --baseurl flag when calling the command line
baseserverurl: https://api.speakeasyapi.dev
```

For additional documentation visit: https://docs.speakeasyapi.dev/docs/using-speakeasy/create-client-sdks/intro

# Ignore Files

The SDK generator will clear the output directory before generating the SDKs, to ensure old files are removed. 
If you have any files you want to keep you can place a ".genignore" file in the root of the output directory.
The ".genignore" file follows the same syntax as a ".gitignore" file.

By default (without a .genignore file/folders) the SDK generator will ignore the following files:
	- gen.yaml
	- .genignore
	- .gitignore
	- .git
	- README.md
	- readme.md
	- LICENSE



## Usage

```
speakeasy generate sdk [flags]
```

### Options

```
  -y, --auto-yes         auto answer yes to all prompts
  -b, --baseurl string   base URL for the api (only required if OpenAPI spec doesn't specify root server URLs
  -d, --debug            enable writing debug files with broken code
  -h, --help             help for sdk
  -l, --lang string      language to generate sdk for (available options: [go, python, typescript, java]) (default "go")
  -o, --out string       path to the output directory
  -s, --schema string    path to the openapi schema
```

### Parent Command

* [speakeasy generate](README.md)	 - Generate Client SDKs, OpenAPI specs from request logs (coming soon) and more
