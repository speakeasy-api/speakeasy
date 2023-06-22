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
  -p, --previous string   the version to get changelogs between this and the target version
  -r, --raw               don't format the output for the terminal
  -s, --specific string   the version to get changelogs for
  -t, --target string     target version to get changelog from (default: the latest change)
```

### Parent Command

* [speakeasy generate sdk](README.md)	 - Generating Client SDKs from OpenAPI specs (csharp, go, java, php, python, ruby, terraform, typescript + more coming soon)
