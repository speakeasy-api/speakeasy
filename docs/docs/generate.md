# generate  
`speakeasy docs generate`  


Use this command to generate content for the SDK docs directory.  

## Details

Use this command to generate content for the SDK docs directory.

## Usage

```
speakeasy docs generate [flags]
```

### Options

```
  -y, --auto-yes             auto answer yes to all prompts
  -c, --compile              automatically compile SDK docs content for a single page doc site
  -d, --debug                enable writing debug files with broken code
  -H, --header string        header key to use if authentication is required for downloading schema from remote URL
  -h, --help                 help for generate
  -l, --langs string         a list of languages to include in SDK Doc generation. Example usage -l go,python,typescript
  -o, --out string           path to the output directory
  -r, --repo string          the repository URL for the SDK Docs repo
  -b, --repo-subdir string   the subdirectory of the repository where the SDK Docs are located in the repo, helps with documentation generation
  -s, --schema string        local filepath or URL for the OpenAPI schema (default "./openapi.yaml")
      --token string         token value to use if authentication is required for downloading schema from remote URL
```

### Parent Command

* [speakeasy docs](README.md)	 - Use this command to generate content, compile, and publish SDK docs.
