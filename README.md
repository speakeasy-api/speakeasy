# The Speakeasy CLI 
![181640742-31ab234a-3b39-432e-b899-21037596b360](https://user-images.githubusercontent.com/68016351/196461357-fcb8d90f-cd67-498e-850f-6146c58d0114.png)

Speakeasy is a complete platform for API Developer Experience. Achieve the vision of self service APIs by moving beyond API docs. Give your API users a seamless onboarding and integration experience in minutes. Today Speakeasy powers: 
- Developer Portals a Service: A best in class interactive portal for your API users to self service key management, request logs, usage and more.   
- Client SDKs as Service: Idiomatic Client SDKs that just work. Generators built from the ground up with a focus on a langauge ergonomics 

## Overview

This CLI is a tool for interacting with the [Speakeasy](https://docs.speakeasyapi.dev/docs/speakeasy-cli/) platform and its various functions:

- Generating idiomatic client SDKs from OpenAPI3 specs:
  * Live: go, python3 
  * Coming soon: typescript(web/server), java, rust, ruby, c#
  
Want to learn more about our generation methodology ? Checkout out this [blog post]() to learn more about our generators. If you're interested in having managed Github repos generated for your SDKs reach out to us [here](https://www.speakeasyapi.dev/request-access) to learn more about our enterprise support model or come chat with us on [Slack](https://join.slack.com/t/speakeasy-dev/shared_invite/zt-1df0lalk5-HCAlpcQiqPw8vGukQWhexw). We'd love to help you build out API dev ex.   

- (Coming Soon) Interacting with the Speakeasy platform to create and manage your developer portal.
  * Create and manage workspaces
  * Manage OpenAPI schemas

## Installation

### Homebrew

```bash
brew install speakeasy-api/homebrew-tap/speakeasy
```

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
* [speakeasy generate](docs/generate/README.md)	 - Generate Client SDKs, OpenAPI specs from request logs (coming soon) and more
* [speakeasy validate](docs/validate/README.md)	 - Validate OpenAPI schemas + more (coming soon)
