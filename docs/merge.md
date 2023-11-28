# merge  
`speakeasy merge`  


Merge multiple OpenAPI documents into a single document  

## Details

Merge multiple OpenAPI documents into a single document, useful for merging multiple OpenAPI documents into a single document for generating a client SDK.
Note: That any duplicate operations, components, etc. will be overwritten by the next document in the list.

## Usage

```
speakeasy merge [flags]
```

### Options

```
  -h, --help                  help for merge
  -o, --out string            path to the output file
  -s, --schemas stringArray   paths to the openapi schemas to merge
```

### Parent Command

* [speakeasy](README.md)	 - The speakeasy CLI tool provides access to the speakeasyapi.dev toolchain
