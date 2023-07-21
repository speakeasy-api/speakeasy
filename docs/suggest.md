# suggest  
`speakeasy suggest`  


Validate an OpenAPI document and get fixes suggested by ChatGPT  

## Details

The "suggest" command validates an OpenAPI spec and uses OpenAI's ChatGPT to suggest fixes to your spec.
You will need to set your OpenAI API key in a OPENAI_API_KEY environment variable. You will also need to authenticate with the Speakeasy API,
you must first create an API key via https://app.speakeasyapi.dev and then set the SPEAKEASY_API_KEY environment variable to the value of the API key.

## Usage

```
speakeasy suggest [flags]
```

### Options

```
  -a, --auto-approve          auto continue through all prompts
  -h, --help                  help for suggest
  -n, --max-suggestions int   maximum number of llm suggestions to fetch, the default is no limit (default -1)
  -m, --model string          model to use when making llm suggestions (gpt-3.5-turbo or gpt-4 recommended) (default "gpt-3.5-turbo")
  -s, --schema string         path to the OpenAPI document
```

### Parent Command

* [speakeasy](README.md)	 - The speakeasy cli tool provides access to the speakeasyapi.dev toolchain
