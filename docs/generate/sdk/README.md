# sdk  
`speakeasy generate sdk`  


Generating Client SDKs from OpenAPI specs (go, java, php, python, terraform, typescript + more coming soon)  

## Details

Using the "speakeasy generate sdk" command you can generate client SDK packages for various languages
that are ready to use and publish to your favorite package registry.

The following languages are currently supported:
	- go
	- java
	- php
	- python
	- terraform
	- typescript
	- more coming soon

By default the command will generate a Go SDK, but you can specify a different language using the --lang flag.
It will also use generic defaults for things such as package name (openapi), etc.

# Configuration

To configure the package of the generated SDKs you can config a "gen.yaml" file in the root of the output directory.

Example gen.yaml file for Go SDK:

```
go:
  packageName: github.com/speakeasy-api/speakeasy-client-sdk-go
  version: 0.1.0
generate:
  # baseServerUrl is optional, if not specified it will use the server URL from the OpenAPI document 
  baseServerUrl: https://api.speakeasyapi.dev 
```

Example gen.yaml file for Python SDK:

```
python:
  packageName: speakeasy-client-sdk-python
  version: 0.1.0
  description: Speakeasy API Client SDK for Python
  author: Speakeasy API
generate:
  # baseServerUrl is optional, if not specified it will use the server URL from the OpenAPI document 
  baseServerUrl: https://api.speakeasyapi.dev 
```

Example gen.yaml file for Typescript SDK:

```
typescript:
  packageName: speakeasy-client-sdk-typescript
  version: 0.1.0
  author: Speakeasy API
generate:
  # baseServerUrl is optional, if not specified it will use the server URL from the OpenAPI document 
  baseServerUrl: https://api.speakeasyapi.dev 
```

Example gen.yaml file for Java SDK:

```
java:
  groupID: dev.speakeasyapi
  artifactID: javasdk
  projectName: speakeasy-client-sdk-java
  version: 0.1.0
generate:
  # baseServerUrl is optional, if not specified it will use the server URL from the OpenAPI document 
  baseServerUrl: https://api.speakeasyapi.dev 
```

Example gen.yaml file for PHP SDK:

```
php:
  packageName: speakeasy-client-sdk-php
  namespace: "speakeasyapi\\sdk"
  version: 0.1.0
generate:
  # baseServerUrl is optional, if not specified it will use the server URL from the OpenAPI document 
  baseServerUrl: https://api.speakeasyapi.dev 
```

For additional documentation visit: https://docs.speakeasyapi.dev/docs/using-speakeasy/create-client-sdks/intro


## Usage

```
speakeasy generate sdk [flags]
```

### Options

```
  -y, --auto-yes                 auto answer yes to all prompts
  -d, --debug                    enable writing debug files with broken code
  -h, --help                     help for sdk
  -i, --installationURL string   the language specific installation URL for installation instructions if the SDK is not published to a package manager
  -l, --lang string              language to generate sdk for (available options: [go, java, php, python, terraform, typescript]) (default "go")
  -o, --out string               path to the output directory
  -p, --published                whether the SDK is published to a package manager or not, determines the type of installation instructions to generate
  -s, --schema string            path to the openapi schema (default "./openapi.yaml")
```

### Parent Command

* [speakeasy generate](../README.md)	 - Generate Client SDKs, OpenAPI specs from request logs (coming soon) and more
### Sub Commands

* [speakeasy generate sdk changelog](changelog.md)	 - Prints information about changes to the SDK generator
* [speakeasy generate sdk version](version.md)	 - Print the version number of the SDK generator
