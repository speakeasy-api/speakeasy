# codeSamples  
`speakeasy generate codeSamples`  


Creates an overlay for a given spec containing x-codeSamples extensions for the given languages.  

## Usage

```
speakeasy generate codeSamples [flags]
```

### Options

```
      --config-path string   the path to the directory containing the gen.yaml file(s) to use (default ".")
  -H, --header string        header key to use if authentication is required for downloading schema from remote URL
  -h, --help                 help for codeSamples
  -l, --langs strings        the languages to generate code samples for (comma-separated list)
      --out string           write directly to a file instead of stdout
  -s, --schema string        the schema to generate code samples for
      --token string         token value to use if authentication is required for downloading schema from remote URL
```

### Options inherited from parent commands

```
      --logLevel string   the log level (available options: [info, warn, error]) (default "info")
```

### Parent Command

* [speakeasy generate](README.md)	 - Generate client SDKs, docsites, and more
