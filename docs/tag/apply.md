# apply  
`speakeasy tag apply`  


Add tags to a given revision of your API. Specific to a registry namespace  

## Usage

```
speakeasy tag apply [flags]
```

### Options

```
  -h, --help                     help for apply
  -n, --namespace-name string    the revision to tag
  -r, --revision-digest string   the revision ID to tag
  -t, --tags strings             A list of tags to apply (comma-separated list)
```

### Options inherited from parent commands

```
      --logLevel string   the log level (available options: [info, warn, error]) (default "info")
```

### Parent Command

* [speakeasy tag](README.md)	 - Add tags to a given revision of your API. Specific to a registry namespace
