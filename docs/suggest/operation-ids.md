# operation-ids  
`speakeasy suggest operation-ids`  


Get suggestions to improve your OpenAPI document's operation IDs  

## Usage

```
speakeasy suggest operation-ids [flags]
```

### Options

```
  -h, --help            help for operation-ids
  -o, --out string      write the suggestion to the specified path
      --overlay         write the suggestion as an overlay to --out, instead of the full document (default: true) (default true)
  -s, --schema string   the schema to transform
      --style string    the style of suggestion to provide (default "resource")
```

### Options inherited from parent commands

```
      --logLevel string   the log level (available options: [info, warn, error]) (default "info")
```

### Parent Command

* [speakeasy suggest](README.md)	 - Automatically improve your OpenAPI document with an LLM
