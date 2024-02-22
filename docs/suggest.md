# suggest  
`speakeasy suggest`  


Validate an OpenAPI document and get fixes suggested by ChatGPT  

## Details

The "suggest" command validates an OpenAPI spec and uses OpenAI's ChatGPT to suggest fixes to your spec.
You can use the Speakeasy OpenAI key within our platform limits, or you may set your own using the OPENAI_API_KEY environment variable. You will also need to authenticate with the Speakeasy API,
you must first create an API key via https://app.speakeasyapi.dev and then set the SPEAKEASY_API_KEY environment variable to the value of the API key.

## Usage

```
speakeasy suggest [flags]
```

### Options

```
  -a, --auto-approve           auto continue through all prompts
      --cache-folder string    caches computations into a given folder
      --example-experiment     enables the example experiment for the suggest command, generating an updated document with examples for all primitives.
  -H, --header string          header key to use if authentication is required for downloading schema from remote URL
  -h, --help                   help for suggest
  -l, --level string           error, warn, or hint. The minimum level of severity to request suggestions for (default "warn")
  -n, --max-suggestions int    maximum number of llm suggestions to fetch, the default is no limit (default -1)
  -m, --model string           model to use when making llm suggestions (gpt-4-0613 recommended) (default "gpt-4-0613")
  -c, --num-specs int          number of specs to run suggest on, the default is no limit (default -1)
  -o, --output-file string     output the modified file with suggested fixes applied to the specified path
  -s, --schema string          local path to a directory containing OpenAPI schema(s), or a single OpenAPI schema, or a remote URL to an OpenAPI schema (default "./openapi.yaml")
      --serial                 do not parallelize requesting suggestions
  -y, --summary                show a summary of the remaining validation errors and their counts
      --token string           token value to use if authentication is required for downloading schema from remote URL
  -v, --validation-loops int   number of times to run the validation loop, the default is no limit (only used in parallelized implementation) (default -1)
```

### Options inherited from parent commands

```
      --logLevel string   the log level (available options: [info, warn, error]) (default "info")
```

### Parent Command

* [speakeasy](README.md)	 - The speakeasy cli tool provides access to the speakeasyapi.dev toolchain
