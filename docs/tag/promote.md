# promote  
`speakeasy tag promote`  


Add tags to a revision in the Registry, based on the most recent workflow run  

## Usage

```
speakeasy tag promote [flags]
```

### Options

```
  -c, --code-samples strings   a list of targets whose code samples should be tagged (comma-separated list)
  -h, --help                   help for promote
  -s, --sources strings        a list of sources whose schema revisions should be tagged (comma-separated list)
  -t, --tags strings           A list of tags to apply (comma-separated list)
```

### Options inherited from parent commands

```
      --logLevel string   the log level (available options: [info, warn, error]) (default "info")
```

### Parent Command

* [speakeasy tag](README.md)	 - Add tags to a given revision of your API. Specific to a registry namespace
