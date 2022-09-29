# speakeasy

- [speakeasy](#speakeasy)
  - [Overview](#overview)
  - [Available Commands](#available-commands)
  - [`generate` Command](#generate-command)
    - [Available Commands](#available-commands-1)
    - [`sdk` Command](#sdk-command)
      - [Configuration](#configuration)
      - [Ignore Files](#ignore-files)

## Overview

The Speakeasy CLI Tool is a command line tool for interacting with the Speakeasy https://www.speakeasyapi.dev/ platform and its various functions including:

- Generating Client SDKs from OpenAPI specs (go, python, typescript(web/server), + more coming soon)
- Interacting with the Speakeasy API to create and manage your API workspaces (coming soon)
- Generating OpenAPI specs from your API traffic (coming soon)
- Validating OpenAPI specs (coming soon)
- Generating Postman collections from OpenAPI Specs  (coming soon)


## Available Commands
- generate: Generate Client SDKs, OpenAPI specs (coming soon) and more (coming soon)
- help: Help about any command

## `generate` Command

The `generate` command provides a set of commands for generating client SDKs, OpenAPI specs (coming soon) and more (coming soon).

### Available Commands
- sdk: Generating Client SDKs from OpenAPI specs (`go`, `python` + more coming soon)

### `sdk` Command

Using the `speakeasy generate sdk` command you can generate client SDK packages for various languages
that are ready to use and publish to your favorite package registry.

The following languages are currently supported:
- `go`
- `python`
- more coming soon

By default the command will generate a Go SDK, but you can specify a different language using the `--lang` flag.
It will also use generic defaults for things such as package name (openapi), etc.

#### Configuration

To configure the pacakge of the generated SDKs you can config a "gen.yaml" file in the root of the output directory.

Example gen.yaml file for Go SDK:

```yaml
go:
  packagename: github.com/speakeasy-api/speakeasy-client-sdk-go
# baseserverurl optional, if not specified it will use the server URL from the OpenAPI spec 
# this can also be provided via the --baseurl flag when calling the command line
baseserverurl: https://api.speakeasyapi.dev
```

Example gen.yaml file for Python SDK:

```yaml
python:
  packagename: speakeasy-client-sdk-python
  version: 0.1.0
  description: Speakeasy API Client SDK for Python
# baseserverurl optional, if not specified it will use the server URL from the OpenAPI spec 
# this can also be provided via the --baseurl flag when calling the command line
baseserverurl: https://api.speakeasyapi.dev
```

#### Ignore Files

The SDK generator will clear the output directory before generating the SDKs, to ensure old files are removed. 
If you have any files you want to keep you can place a ".genignore" file in the root of the output directory.
The ".genignore" file follows the same syntax as a [".gitignore"](https://git-scm.com/docs/gitignore) file.

By default (without a .genignore file/folders) the SDK generator will ignore the following files:
- `gen.yaml`
- `.genignore`
- `.gitignore`
- `.git`
- `README.md`
- `readme.md`
