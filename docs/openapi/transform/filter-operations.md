# filter-operations  
`speakeasy openapi transform filter-operations`  


Given an OpenAPI file, filter down to just the given set of operations  

## Usage

```
speakeasy openapi transform filter-operations [flags]
```

### Options

```
  -x, --exclude              exclude the given operationIDs, rather than including them
  -h, --help                 help for filter-operations
      --operations strings   list of operation IDs to include (or exclude) (comma-separated list)
  -o, --out string           write directly to a file instead of stdout
  -s, --schema string        the schema to transform
```

### Options inherited from parent commands

```
      --logLevel string   the log level (available options: [info, warn, error]) (default "info")
```

### Parent Command

* [speakeasy openapi transform](README.md)	 - Transform an OpenAPI spec using a well-defined function
