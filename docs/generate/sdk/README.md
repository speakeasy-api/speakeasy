# sdk  
`speakeasy generate sdk`  


Generating Client SDKs from OpenAPI specs (csharp, go, java, php, python, ruby, swift, terraform, typescript, unity)  

## Details

Using the "speakeasy generate sdk" command you can generate client SDK packages for various languages
that are ready to use and publish to your favorite package registry.

The following languages are currently supported:
	- csharp
	- go
	- java
	- php
	- python
	- ruby
	- swift
	- terraform
	- typescript
	- unity

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

Example gen.yaml file for C# SDK:

```
csharp:
  version: 0.1.0
  author: Speakeasy
  maxMethodParams: 0
  packageName: SpeakeasySDK
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
      --force                    Force generation of SDKs even when no changes are present
  -H, --header string            header key to use if authentication is required for downloading schema from remote URL
  -h, --help                     help for sdk
  -i, --installationURL string   the language specific installation URL for installation instructions if the SDK is not published to a package manager
  -l, --lang string              language to generate sdk for (available options: [csharp, go, java, php, python, ruby, swift, terraform, typescript, unity]) (default "go")
  -o, --out string               path to the output directory
  -p, --published                whether the SDK is published to a package manager or not, determines the type of installation instructions to generate
  -r, --repo string              the repository URL for the SDK, if the published (-p) flag isn't used this will be used to generate installation instructions
  -b, --repo-subdir string       the subdirectory of the repository where the SDK is located in the repo, helps with documentation generation
  -s, --schema string            local filepath or URL for the OpenAPI schema (default "./openapi.yaml")
      --token string             token value to use if authentication is required for downloading schema from remote URL
```

### Options inherited from parent commands

```
      --logLevel string   the log level (available options: [info, warn, error]) (default "info")
```

### Parent Command

* [speakeasy generate](../README.md)	 - One off Generations for client SDKs, docsites, and more
### Sub Commands

* [speakeasy generate sdk changelog](changelog.md)	 - Prints information about changes to the SDK generator
* [speakeasy generate sdk version](version.md)	 - Print the version number of the SDK generator
