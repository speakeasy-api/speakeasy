# The Speakeasy CLI - Generate Client SDKs Like a Human Wrote Them
![181640742-31ab234a-3b39-432e-b899-21037596b360](https://user-images.githubusercontent.com/68016351/196461357-fcb8d90f-cd67-498e-850f-6146c58d0114.png)

[Speakeasy](https://www.speakeasyapi.dev/) is a complete platform for API Developer Experience. Achieve the vision of self service APIs by moving beyond API docs. Give your API users a seamless onboarding and integration experience in minutes. Use this CLI to generate and manage Idiomatic Client SDKs that just work. The CLI invokes generators built from the ground up with a focus on a langauge ergonomics and extensibility.

![ezgif-1-941c72f269](https://user-images.githubusercontent.com/68016351/206042347-a3dc40de-4339-4b88-9fff-39513c1c8216.gif)

## Overview

This CLI is a tool for interacting with the [Speakeasy](https://docs.speakeasyapi.dev/docs/speakeasy-cli/) platform and its various functions:

* Generating idiomatic client SDKs from OpenAPI3 specs:
  * Live: Go, Python3, Typescript(Node), Java (alpha)
  * Coming soon: Terraform, Rust, Ruby, C# and more languages on upon request! 
  
* Validating the correctness of OpenAPI3 specs. The CLI has a built in command to validate your spec and post helpful error messages. 

## Design Choices

All the SDKs we generate are designed to be as idiomatic to the language they are generated for as possible while being similar enough to each other to allow some familarity between them, but also to allow for an effiecient generation engine that is capabale of supporting many languages. Some of the design decisions we made are listed below:

* Each of the SDKs generally implement a base SDK class that contains the methods for each of the API endpoints defined in a spec.
* Where possible we generate fully typed models from the OpenAPI document and seperate those models defined as components in the docs and those that are defined inline with operations.
* We use reflection metadata where possible to annotate types with the required metadata needed to determine how to serialize and deserialize them, based on the configuration in the OpenAPI document.
* We generate full packages for each language that should be able to be published to a package registry with little additional work, to get them in your end-users hands as quickly as possible. If you're interested in having a managed pipeline to your package manager check out our Github action. 

Want to learn more about our methodology? Here is a [blog post](https://www.speakeasyapi.dev/post/client-sdks-as-a-service) to learn more about our generators as compared to the OSS options. If you're interested in having managed Github repos generated for your SDKs or enterprise support reach out to us [here](https://www.speakeasyapi.dev/request-access) or [come chat with us](https://calendly.com/d/drw-t98-rpq/simon-sagar-speakeasy). We'd love to help you build out API dev ex.

## Installation

### Homebrew

```bash
brew install speakeasy-api/homebrew-tap/speakeasy
```

## SDK Generation

**Command**:
```
speakeasy generate sdk [flags]
```
**Options**:
```
  -b, --baseurl string   base URL for the api (only required if OpenAPI spec doesn't specify root server URLs
  -d, --debug            enable writing debug files with broken code
  -h, --help             help for sdk
  -l, --lang string      language to generate sdk for (available options: [go, python, typescript, java]) (default "go")
  -o, --out string       path to the output directory
  -s, --schema string    path to the openapi schema
  -y, --auto-yes string  auto answer yes to all prompts
```

For in depth documentation please see our [docs](https://docs.speakeasyapi.dev/docs/speakeasy-cli/getting-started). 

## Schema Validation

**Command**:
```
speakeasy validate openapi [flags]
```
**Options**:
```
  -h, --help            help for openapi
  -s, --schema string   path to the openapi schema
```

## OpenAPI Support

* [ ] Global and per method ServerURL configuration (include base url and templating) - https://swagger.io/docs/specification/api-host-and-base-path/
* [ ] Global and per method Security configuration - https://swagger.io/docs/specification/authentication/
* [ ] Method generation
* [ ] Request/Response Model Generation
* [ ] Path Param Serialization - https://swagger.io/docs/specification/describing-parameters/#path-parameters
  * [ ] Default Path Paramater Serialization `(style = simple, explode = false)` - https://swagger.io/docs/specification/serialization/#path
  * [ ] Basic types and simple objects only currently supported
  * [ ] Other styles not currently supported
* [ ] Query Param Serialization - https://swagger.io/docs/specification/describing-parameters/#query-parameters & https://swagger.io/docs/specification/serialization/#query
  * [ ] `json` serialization
  * [ ] `form` style serialization
    * [ ] Basic types and simple objects only currently supported
  * [ ] `deepObject` style serialization
  * [ ] Other styles not currently supported
* [ ] Request Headers - https://swagger.io/docs/specification/serialization/#header
  * [ ] Including explode handling
* [ ] Request Body Serialization
  * [ ] Multipart Encoding - https://swagger.io/docs/specification/describing-request-body/multipart-requests/
    * [ ] Binary file support
    * [ ] Form data support
    * [ ] Encoding not supported
  * [ ] JSON Serialization
  * [ ] x-www-form-urlencoded Serialization - https://swagger.io/docs/specification/describing-request-body
    * [ ] Including encoding
    * [ ] Doesn't support non-object types
  * [ ] plain text / string serialization
  * [ ] raw byte serialization
  * [ ] Other serialization not currently supported
  * [ ] Handling of `required` body
* [ ] Response Body Serialization
  * [ ] Return StatusCode and Content-Type
  * [ ] plain text / string deserialization
  * [ ] raw byte deserialization
  * [ ] Json deserialization
  * [ ] Other deserialization not currently supported
* [ ] Media-type patterns - https://swagger.io/docs/specification/media-types/
* [ ] Full openapi datatype support
  * [ ] Basic types - https://swagger.io/docs/specification/data-models/data-types/
  * [ ] Enums
  * [ ] Number formats ie float, double, int32, int64
  * [ ] Date-time
  * [ ] Binary
  * [ ] Arrays
  * [ ] Objects
  * [ ] Optional
  * [ ] Maps
  * [ ] Any type
  * [ ] OneOf/AnyOf/AllOf - https://swagger.io/docs/specification/data-models/oneof-anyof-allof-not/
* [ ] Auxiliary files
  * [ ] Utilities classes/functions to help with serialization/deserialization.
  * [ ] Files needed for creating a fully compilable package that can be published to the relevant package manager without further changes.
* [ ] Support for x-speakeasy-server-id map generation
* [ ] Support for snippet generation
* [ ] Support for readme generation

## Advanced Generation Features

* [SDK Gen Configuration](https://docs.speakeasyapi.dev/docs/using-speakeasy/create-client-sdks/configuration/index.html) - Learn how to configure the SDK generator to your needs.
* [Generated Comments](https://docs.speakeasyapi.dev/docs/using-speakeasy/create-client-sdks/generated-comments/index.html) - Learn how comments are generated from your OpenAPI document and how to customize them.
* [Configuring the SDK with Server URLs](https://docs.speakeasyapi.dev/docs/using-speakeasy/create-client-sdks/server-urls/index.html) - Learn how to configure the SDK to use different server URLs for different environments.
* [Readme Generation](https://docs.speakeasyapi.dev/docs/using-speakeasy/create-client-sdks/readme-generation/index.html) - Learn how the SDK generates README.md files and how to control this.
* [Using custom HTTPs Clients with the SDK](https://docs.speakeasyapi.dev/docs/using-speakeasy/create-client-sdks/custom-http-client/index.html) - Learn how to provide a custom HTTP Client to the SDKs at runtime.
* [Capturing Telemetry on SDK Usage](https://docs.speakeasyapi.dev/docs/using-speakeasy/create-client-sdks/capturing-telemetry/index.html) - Learn how you can capture telemetry to get an understanding of how your SDKs are being used.
* [Automated SDK Generation](https://docs.speakeasyapi.dev/docs/using-speakeasy/create-client-sdks/automate-sdks/index.html) - Use our Github Action and Workflows to setup CI/CD for generating and publishing your SDKs.

<!-- WARNING: The below content is replaced by running `go run cmd/docs/main.go` please don't manually edit anything below this line -->
## CLI  
`speakeasy`  


The speakeasy cli tool provides access to the speakeasyapi.dev toolchain  

### Details

 A cli tool for interacting with the Speakeasy https://www.speakeasyapi.dev/ platform and its various functions including:
	- Generating Client SDKs from OpenAPI specs (go, python, typescript(web/server), + more coming soon)
	- Interacting with the Speakeasy API to create and manage your API workspaces	(coming soon)
	- Generating OpenAPI specs from your API traffic 								(coming soon)
	- Validating OpenAPI specs 														(coming soon)
	- Generating Postman collections from OpenAPI Specs 							(coming soon)


### Usage

```
speakeasy [flags]
```

#### Options

```
  -h, --help   help for speakeasy
```

#### Sub Commands

* [speakeasy api](docs/api/README.md)	 - Access the Speakeasy API via the CLI
* [speakeasy auth](docs/auth/README.md)	 - Authenticate the CLI
* [speakeasy generate](docs/generate/README.md)	 - Generate Client SDKs, OpenAPI specs from request logs (coming soon) and more
* [speakeasy validate](docs/validate/README.md)	 - Validate OpenAPI documents + more (coming soon)
