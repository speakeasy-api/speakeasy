# usage  
`speakeasy usage`  


Output usage information for a given OpenAPI schema to a CSV  

## Details

Output usage information containing counts of OpenAPI features for a given OpenAPI schema to a CSV

## Usage

```
speakeasy usage [flags]
```

### Options

```
  -d, --debug           enable writing debug files with broken code
  -H, --header string   header key to use if authentication is required for downloading schema from remote URL
  -h, --help            help for usage
  -o, --out string      Path to output file
  -s, --schema string   local filepath or URL for the OpenAPI schema (default "./openapi.yaml")
      --token string    token value to use if authentication is required for downloading schema from remote URL
```

### Parent Command

* [speakeasy](README.md)	 - The speakeasy cli tool provides access to the speakeasyapi.dev toolchain
