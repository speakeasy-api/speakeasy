# generate  
`speakeasy generate`  


One off Generations for client SDKs and more  

## Details

The "generate" command provides a set of commands for one off generations of client SDKs and Terraform providers

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

* [speakeasy](../README.md)	 - The Speakeasy CLI tool provides access to the Speakeasy.com platform
### Sub Commands

* [speakeasy generate changelog](changelog.md)	 - Prints information about changes to the SDK generator
* [speakeasy generate codeSamples](codeSamples.md)	 - Creates an overlay for a given spec containing x-codeSamples extensions for the given languages.
* [speakeasy generate sdk](sdk/README.md)	 - Generating Client SDKs from OpenAPI specs (csharp, go, java, php, postman, python, ruby, swift, terraform, typescript, unity)
* [speakeasy generate usage](usage.md)	 - Generate standalone usage snippets for SDKs in (go, typescript, python, java, php, swift, ruby, csharp, unity)
* [speakeasy generate version](version.md)	 - Print the version number of the SDK generator
