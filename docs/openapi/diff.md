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
  -h, --help            help for diff
      --new string      local filepath or URL for the updated OpenAPI schema
      --old string      local filepath or URL for the base OpenAPI schema to compare against
  -o, --output string   how to visualize the diff (one of: summary, console, html, default: summary) (default "summary")
```

### Options inherited from parent commands

```
      --logLevel string   the log level (available options: [info, warn, error]) (default "info")
```

### Parent Command

* [speakeasy openapi](README.md)	 - Validate and compare OpenAPI documents
