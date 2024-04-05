# docs  
`speakeasy generate docs`  


Use this command to generate content for the SDK docs directory.  

## Details

Use this command to generate content for the SDK docs directory.

## Usage

```
speakeasy generate docs [flags]
```

### Options

```
  -y, --auto-yes             auto answer yes to all prompts
  -c, --compile              automatically compile SDK docs content for a single page doc site
  -d, --debug                enable writing debug files with broken code
  -H, --header string        header key to use if authentication is required for downloading schema from remote URL
  -h, --help                 help for docs
  -l, --langs string         a list of languages to include in SDK Docs generation. Example usage -l go,python,typescript
  -o, --out string           path to the output directory
  -r, --repo string          the repository URL for the SDK, if the published (-p) flag isn't used this will be used to generate installation instructions
  -b, --repo-subdir string   the subdirectory of the repository where the SDK is located in the repo, helps with documentation generation
  -s, --schema string        local filepath or URL for the OpenAPI schema (default "./openapi.yaml")
      --token string         token value to use if authentication is required for downloading schema from remote URL
```

### Options inherited from parent commands

```
      --logLevel string   the log level (available options: [info, warn, error]) (default "info")
```

### Parent Command

* [speakeasy generate](README.md)	 - One off Generations for client SDKs, docsites, and more
