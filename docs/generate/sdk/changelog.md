# changelog  
`speakeasy generate sdk changelog`  


Prints information about changes to the SDK generator  

## Details

Prints information about changes to the SDK generator with the ability to filter by version and format the output for the terminal or parsing. By default it will print the latest changelog entry.

## Usage

```
speakeasy generate sdk changelog [flags]
```

### Options

```
  -h, --help              help for changelog
  -l, --language string   the language to get changelogs for, if not specified the changelog for the generator itself will be returned
  -p, --previous string   the version(s) to get changelogs between this and the target version(s)
  -r, --raw               don't format the output for the terminal
  -s, --specific string   the version to get changelogs for, not used if language is specified
  -t, --target string     the version(s) to get changelogs from, if not specified the latest version(s) will be used
```

### Options inherited from parent commands

```
      --logLevel string   the log level (available options: [info, warn, error]) (default "info")
```

### Parent Command

* [speakeasy generate sdk](README.md)	 - Generating Client SDKs from OpenAPI specs (csharp, go, java, php, postman, python, ruby, swift, terraform, typescript, unity)
