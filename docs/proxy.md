# proxy  
`speakeasy proxy`  


Proxy provides a reverse-proxy for debugging and testing Speakeasy's Traffic Capture capabilities  

## Details

Proxy provides a reverse-proxy for debugging and testing Speakeasy's Traffic Capture capabilities

## Usage

```
speakeasy proxy [flags]
```

### Options

```
  -a, --api-id string       the API ID to send captured traffic to
  -d, --downstream string   the downstream base url to proxy traffic to
  -h, --help                help for proxy
  -p, --port string         port to run the proxy on (default "3333")
  -s, --schema string       path to an openapi document that can be used to match incoming traffic to API endpoints
  -v, --version-id string   the Version ID to send captured traffic to
```

### Options inherited from parent commands

```
      --logLevel string   the log level (available options: [info, warn, error]) (default "info")
```

### Parent Command

* [speakeasy](README.md)	 - The speakeasy cli tool provides access to the speakeasyapi.dev toolchain
