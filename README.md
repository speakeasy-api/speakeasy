# The Speakeasy CLI - Generate Client SDKs Like a Human Wrote Them

![181640742-31ab234a-3b39-432e-b899-21037596b360](https://user-images.githubusercontent.com/68016351/196461357-fcb8d90f-cd67-498e-850f-6146c58d0114.png)

Speakeasy is the fastest way to ship developer experience for your APIs.

![ezgif com-video-to-gif (1)](https://github.com/speakeasy-api/speakeasy/assets/90289500/ff6972c4-4e8a-4a5c-a976-75e97cc42f5a)

## What is Speakeasy ?

[Speakeasy](https://www.speakeasyapi.dev/) gives your users the DevEx that makes API integrations easy. Don't put the burden of integration on your users. Take your APIs to market with best in class sdks and a complete self-service experience from shipping great sdks to managing keys, logs and more.

## What is the Speakeasy CLI ?

This CLI is a tool for interacting with the [Speakeasy](https://docs.speakeasyapi.dev/docs/speakeasy-cli/) platform - the CLI brings the functionality of Speakeasy into your development workflow. It can be run locally or in your CI/CD pipeline to validate your API specs, generate SDKs and more.

Current functions of the CLI include:

* Generating idiomatic client SDKs from OpenAPI 3.X specs:
  * Live: Go, Python3, Typescript(Node), Java, PHP, Ruby, Terraform
  * Coming soon: Rust, C#, Swift and more languages upon request!
* Validating the correctness of OpenAPI 3.X specs. The CLI has a built in command to validate your spec and post helpful error messages.
* Merging OpenAPI 3.X specs. The CLI can merge multiple OpenAPI 3.X specs into a single spec.
* Authenticating with the platform and managing API keys.
* Using the Speakeasy API to manage your integration.

## Design Choices

All the SDKs we generate are designed to be as idiomatic to the language they are generated for as possible while being similar enough to each other to allow some familiarity between them, but also to allow for an efficient generation engine that is capable of supporting many languages. Some of the design decisions we made are listed below:

* Each of the SDKs generally implement a base SDK class that contains the methods for each of the API endpoints defined in the OpenAPI document.
* Where possible we generate fully typed models from the OpenAPI document and separate those models defined as components in the docs and those that are defined inline with operations.
* We use reflection metadata where possible to annotate types with the required metadata needed to determine how to serialize and deserialize them, based on the configuration in the OpenAPI document.
* We generate full packages for each language that should be able to be published to a package registry with little additional work, to get them in your end-users hands as quickly as possible. If you're interested in having a managed pipeline to your package manager check out our Github action.

Want to learn more about our methodology? Here is a [blog post](https://www.speakeasyapi.dev/post/client-sdks-as-a-service) to learn more about our generators as compared to the OSS options. If you're interested in having managed Github repos generated for your SDKs or enterprise support reach out to us [here](https://www.speakeasyapi.dev/request-access) or [come chat with us](https://calendly.com/d/drw-t98-rpq/simon-sagar-speakeasy). We'd love to help you build out API dev ex.

> We may capture telemetry on usage of the CLI to better understand API (OpenAPI) features so that we can build better code generators and other tools over time

## Installation

### Homebrew (MacOS and Linux)

```bash
brew install speakeasy-api/homebrew-tap/speakeasy
```

### Chocolatey (Windows)

```cmd
choco install speakeasy
```

### Manual Installation

Download the latest release for your platform from the [releases page](https://github.com/speakeasy-api/speakeasy/releases), extract and add the binary to your path.

### Keeping up to date

The CLI will warn you if you're running an out of date version. To update the CLI run:

```bash
speakeasy update
```

or install the latest version via your package manager.

## Getting Started with the Speakeasy CLI

Once you installed the Speakeasy CLI, you can verify it's working by running:

```bash
speakeasy --help
```

See the [docs](https://docs.speakeasyapi.dev/docs/speakeasy-cli/getting-started) for more information on how to get started with the Speakeasy CLI.

### Authenticating Speakeasy CLI

Speakeasy CLI depends on Speakeasy Platform APIs. Connect your Speakeasy CLI with Speakeasy Platform by running:

```bash
speakeasy auth login
```

You'll be redirected to a login URL to select an existing workspace or create a new workspace on the platform. If you're local network prevents 
accessing the login page prompted by the CLI you can login manually at [app.speakeasyapi.dev](https://app.speakeasyapi.dev), retrieve an API key and populate a local environment
variable named `SPEAKEASY_API_KEY` with the key.

<img width="1268" alt="Screenshot 2023-01-29 at 23 12 05" src="https://user-images.githubusercontent.com/68016351/215410983-b41dab8c-12b1-472c-a2fb-3325b881ff8e.png">

### SDK Generation

**Command**:

```bash
speakeasy generate sdk [flags]
```

**Options**:

```bash
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

Note, Schema validation doesn't require logging in to the Speakeasy Platform.

**Command**:

```bash
speakeasy validate openapi [flags]
```

**Options**:

```bash
  -h, --help            help for openapi
  -s, --schema string   path to the openapi schema
```

## OpenAPI Usage

Note, OpenAPI usage doesn't require logging in to the Speakeasy Platform.

The following command outputs usage information containing counts of OpenAPI features for a given OpenAPI schema to a CSV.

**Command**:

```bash
speakeasy usage [flags]
```

**Options**:

```bash
  -d, --debug         enable writing debug files with broken code
  -f, --file string   Path to file to generate usage information for
  -h, --help          help for usage
  -o, --out string    Path to output file
```

## OpenAPI Support

* [ ] Global and per method ServerURL configuration (include base url and templating) - <https://swagger.io/docs/specification/api-host-and-base-path/>
* [ ] Global and per method Security configuration - <https://swagger.io/docs/specification/authentication/>
* [ ] Method generation
* [ ] Request/Response Model Generation
* [ ] Path Param Serialization - <https://swagger.io/docs/specification/describing-parameters/#path-parameters>
  * [ ] Default Path Paramater Serialization `(style = simple, explode = false)` - <https://swagger.io/docs/specification/serialization/#path>
  * [ ] Basic types and simple objects only currently supported
  * [ ] Other styles not currently supported
* [ ] Query Param Serialization - <https://swagger.io/docs/specification/describing-parameters/#query-parameters> & <https://swagger.io/docs/specification/serialization/#query>
  * [ ] `json` serialization
  * [ ] `form` style serialization
    * [ ] Basic types and simple objects only currently supported
  * [ ] `deepObject` style serialization
  * [ ] Other styles not currently supported
* [ ] Request Headers - <https://swagger.io/docs/specification/serialization/#header>
  * [ ] Including explode handling
* [ ] Request Body Serialization
  * [ ] Multipart Encoding - <https://swagger.io/docs/specification/describing-request-body/multipart-requests/>
    * [ ] Binary file support
    * [ ] Form data support
    * [ ] Encoding not supported
  * [ ] JSON Serialization
  * [ ] x-www-form-urlencoded Serialization - <https://swagger.io/docs/specification/describing-request-body>
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
* [ ] Media-type patterns - <https://swagger.io/docs/specification/media-types/>
* [ ] Full OpenAPI datatype support
  * [ ] Basic types - <https://swagger.io/docs/specification/data-models/data-types/>
  * [ ] Enums
  * [ ] Number formats ie float, double, int32, int64
  * [ ] Date-time
  * [ ] Binary
  * [ ] Arrays
  * [ ] Objects
  * [ ] Optional
  * [ ] Maps
  * [ ] Any type
  * [ ] OneOf/AnyOf/AllOf - <https://swagger.io/docs/specification/data-models/oneof-anyof-allof-not/>
* [ ] Auxiliary files
  * [ ] Utilities classes/functions to help with serialization/deserialization.
  * [ ] Files needed for creating a fully compilable package that can be published to the relevant package manager without further changes.
* [ ] Support for x-speakeasy-server-id map generation
* [ ] Support for snippet generation
* [ ] Support for readme generation

## Advanced Generation Features

* [SDK Gen Configuration](https://speakeasyapi.dev/docs/using-speakeasy/create-client-sdks/customize-sdks/intro/) - Learn how to configure the SDK generator to your needs.
* [Generated Comments](https://speakeasyapi.dev/docs/using-speakeasy/create-client-sdks/sdk-docs/#comments) - Learn how comments are generated from your OpenAPI document and how to customize them.
* [Configuring the SDK with Server URLs](https://speakeasyapi.dev/docs/using-speakeasy/create-client-sdks/customize-sdks/servers/) - Learn how to configure the SDK to use different server URLs for different environments.
* [Readme Generation](https://speakeasyapi.dev/docs/using-speakeasy/create-client-sdks/sdk-docs/) - Learn how the SDK generates README.md files and how to control this.
* [Using custom HTTPs Clients with the SDK](https://speakeasyapi.dev/docs/using-speakeasy/create-client-sdks/customize-sdks/custom-http-client/) - Learn how to provide a custom HTTP Client to the SDKs at runtime.
* [Capturing Telemetry on SDK Usage](https://speakeasyapi.dev/docs/using-speakeasy/create-client-sdks/sdk-telemetry/) - Learn how you can capture telemetry to get an understanding of how your SDKs are being used.
* [Automated SDK Generation](https://speakeasyapi.dev/docs/using-speakeasy/create-client-sdks/github-setup/) - Use our Github Action and Workflows to setup CI/CD for generating and publishing your SDKs.
* [Override Generated Names](https://speakeasyapi.dev/docs/using-speakeasy/create-client-sdks/customize-sdks/methods/) - Speakeasy uses your OpenAPI schema to infer names for class types, methods, and parameters. However, you can override these names to tailor the generated SDK to your preferences.
* [Add retries to your SDKs](https://speakeasyapi.dev/docs/using-speakeasy/create-client-sdks/customize-sdks/retries/) - The generator supports the ability to generate SDKs that will automatically retry requests that fail due to network errors or any configured HTTP Status code.

## Getting Support

If you need support using Speakeasy CLI, please contact us via [email](info@speakeasyapi.dev), [slack](https://join.slack.com/t/speakeasy-dev/shared_invite/zt-1df0lalk5-HCAlpcQiqPw8vGukQWhexw) or file a Github issue and we'll respond ASAP !

<!-- WARNING: The below content is replaced by running `go run cmd/docs/main.go` please don't manually edit anything below this line -->

### Usage

```bash
speakeasy [flags]
```

#### Options

```bash
  -h, --help   help for speakeasy
```

#### Sub Commands

* [speakeasy api](docs/api/README.md)  - Access the Speakeasy API via the CLI
* [speakeasy auth](docs/auth/README.md)  - Authenticate the CLI
* [speakeasy generate](docs/generate/README.md)  - Generate Client SDKs, OpenAPI specs from request logs (coming soon) and more
* [speakeasy validate](docs/validate/README.md)  - Validate OpenAPI documents + more (coming soon)

## CLI  
`speakeasy`  


The speakeasy cli tool provides access to the speakeasyapi.dev toolchain  

### Details

 A cli tool for interacting with the Speakeasy https://www.speakeasyapi.dev/ platform and its various functions including:
	- Generating Client SDKs from OpenAPI specs (go, python, typescript, java, php + more coming soon)
	- Validating OpenAPI specs
	- Interacting with the Speakeasy API to create and manage your API workspaces
	- Generating OpenAPI specs from your API traffic 								(coming soon)
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
* [speakeasy merge](docs/merge.md)	 - Merge multiple OpenAPI documents into a single document
* [speakeasy proxy](docs/proxy.md)	 - Proxy provides a reverse-proxy for debugging and testing Speakeasy's Traffic Capture capabilities
* [speakeasy suggest](docs/suggest.md)	 - Validate an OpenAPI document and get fixes suggested by ChatGPT
* [speakeasy update](docs/update.md)	 - Update the Speakeasy CLI to the latest version
* [speakeasy usage](docs/usage.md)	 - Output usage information for a given OpenAPI schema to a CSV
* [speakeasy validate](docs/validate/README.md)	 - Validate OpenAPI documents + more (coming soon)
