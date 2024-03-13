# overlay  
`speakeasy overlay`  


Work with OpenAPI Overlays  

## Usage

```bash
speakeasy overlay [flags]
```

### Options

```sql
  -h, --help   help for overlay
```

### Options inherited from parent commands

```sql
      --logLevel string   the log level (available options: [info, warn, error]) (default "info")
```

### Parent Command

* [speakeasy](../README.md) - The speakeasy cli tool provides access to the speakeasyapi.dev toolchain
### Sub Commands

* [speakeasy overlay apply](apply.md) - Given an overlay, construct a new specification by extending a specification and applying the overlay, and output it to stdout.
* [speakeasy overlay compare](compare.md) - Given two specs, output an overlay that describes the differences between them
* [speakeasy overlay validate](validate.md) - Given an overlay, validate it according to the OpenAPI Overlay specification
