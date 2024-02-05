# openapi  
`speakeasy validate openapi`  


Validate an OpenAPI document  

## Details

Validates an OpenAPI document is valid and conforms to the Speakeasy OpenAPI specification.

## Usage

```
speakeasy validate openapi [flags]
```

### Options

```
  -H, --header string                 header key to use if authentication is required for downloading schema from remote URL
  -h, --help                          help for openapi
      --max-validation-errors int     limit the number of errors to output (default 1000, 0 = no limit) (default 1000)
      --max-validation-warnings int   limit the number of warnings to output (default 1000, 0 = no limit) (default 1000)
  -o, --output-hints                  output validation hints in addition to warnings/errors
  -s, --schema string                 local filepath or URL for the OpenAPI schema
      --token string                  token value to use if authentication is required for downloading schema from remote URL
```

### Options inherited from parent commands

```
      --logLevel string   the log level (available options: [info, warn, error]) (default "info")
```

### Parent Command

* [speakeasy validate](README.md)	 - Validate OpenAPI documents
