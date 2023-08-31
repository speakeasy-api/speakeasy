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
  -p, --previous string   the version to get changelogs between this and the target version
  -r, --raw               don't format the output for the terminal
  -s, --specific string   the version to get changelogs for, not used if language is specified
  -t, --target string     target version to get changelog from (required if language is specified otherwise defaults to latest version of the generator)
```

### Parent Command

* [speakeasy generate sdk](README.md)	 - Generating Client SDKs from OpenAPI specs (csharp, go, java, php, python, ruby, swift, terraform, typescript, unity + more coming soon)
