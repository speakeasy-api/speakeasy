# diff  
`speakeasy openapi diff`  


Visualize the changes between two OpenAPI documents  

## Details

Visualize the changes between two OpenAPI documents

## Usage

```
speakeasy openapi diff [flags]
```

### Options

```
  -f, --format string   output format (one of: summary, console, html, default: summary) (default "summary")
  -h, --help            help for diff
      --new string      local filepath or URL for the updated OpenAPI schema
      --old string      local filepath or URL for the base OpenAPI schema to compare against
  -o, --output string   output file (default "-")
```

### Options inherited from parent commands

```
      --logLevel string   the log level (available options: [info, warn, error]) (default "info")
```

### Parent Command

* [speakeasy openapi](README.md)	 - Utilities for working with OpenAPI documents
