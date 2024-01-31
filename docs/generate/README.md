# generate  
`speakeasy generate`  


Generate client SDKs, docsites, and more  

## Details

The "generate" command provides a set of commands for generating client SDKs, OpenAPI specs (coming soon) and more (coming soon).

## Usage

```
speakeasy generate [flags]
```

### Options

```
  -h, --help   help for generate
```

### Options inherited from parent commands

```
      --logLevel string   the log level (available options: [info, warn, error]) (default "info")
```

### Parent Command

* [speakeasy](../README.md)	 - The speakeasy cli tool provides access to the speakeasyapi.dev toolchain
### Sub Commands

* [speakeasy generate changelog](changelog.md)	 - Prints information about changes to the SDK generator
* [speakeasy generate docs](docs.md)	 - Use this command to generate content for the SDK docs directory.
* [speakeasy generate sdk](sdk/README.md)	 - Generating Client SDKs from OpenAPI specs (csharp, go, java, php, python, ruby, swift, terraform, typescript, unity + more coming soon)
* [speakeasy generate usage](usage.md)	 - Generate standalone usage snippets for SDKs in (go, typescript, python, java, php, swift, ruby, csharp, unity)
* [speakeasy generate version](version.md)	 - Print the version number of the SDK generator
