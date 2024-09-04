# overlay  
`speakeasy overlay`  


Work with OpenAPI Overlays  

## Details

# Overlay

Command group for working with OpenAPI Overlays.


## Usage

```
speakeasy overlay [flags]
```

### Options

```
  -h, --help   help for overlay
```

### Options inherited from parent commands

```
      --logLevel string   the log level (available options: [info, warn, error]) (default "info")
```

### Parent Command

* [speakeasy](../README.md)	 - The Speakeasy CLI tool provides access to the Speakeasy.com platform
### Sub Commands

* [speakeasy overlay apply](apply.md)	 - Given an overlay, construct a new specification by extending a specification and applying the overlay, and output it to stdout.
* [speakeasy overlay compare](compare.md)	 - Given two specs (before and after), output an overlay that describes the differences between them
* [speakeasy overlay validate](validate.md)	 - Given an overlay, validate it according to the OpenAPI Overlay specification
