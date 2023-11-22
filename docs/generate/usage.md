# usage  
`speakeasy generate usage`  


Generate standalone usage snippets for SDKs in (go, typescript, python, java, php, swift, ruby, csharp, unity)  

## Details

Using the "speakeasy generate usage" command you can generate usage snippets for various SDKs.

The following languages are currently supported:
	- go
	- typescript
	- python
	- java
	- php
	- swift
	- ruby
	- csharp
	- unity
	- more coming soon

You can generate usage snippets by OperationID or by Namespace. By default this command will write to stdout.

You can also select to write to a file or write to a formatted output directory.


## Usage

```
speakeasy generate usage [flags]
```

### Options

```
  -c, --config-path string    An optional argument to pass in the path to a directory that holds the gen.yaml configuration file. (default ".")
  -H, --header string         header key to use if authentication is required for downloading schema from remote URL
  -h, --help                  help for usage
  -l, --lang string           language to generate sdk for (available options: [csharp, go, java, php, python, ruby, swift, terraform, typescript, unity]) (default "go")
  -n, --namespace string      The namespace to generate multiple usage snippets for. This could correspond to a tag or a x-speakeasy-group-name in your OpenAPI spec.
  -i, --operation-id string   The OperationID to generate usage snippet for
  -o, --out string            By default this command will write to stdout. If a filepath is provided results will be written into that file.
                              	If the path to an existing directory is provided, all results will be formatted into that directory with each operation getting its own sub folder.
  -s, --schema string         path to the openapi schema (default "./openapi.yaml")
      --token string          token value to use if authentication is required for downloading schema from remote URL
```

### Parent Command

* [speakeasy generate](README.md)	 - Generate Client SDKs, OpenAPI specs from request logs (coming soon) and more
