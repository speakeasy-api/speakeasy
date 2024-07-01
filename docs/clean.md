# clean  
`speakeasy clean`  


Speakeasy clean can be used to clean up cache, stale temp folders, and old CLI binaries.  

## Details

Using speakeasy clean outside of an SDK directory or with the --global will clean cache, CLI binaries, and more out of the root .speakeasy folder.
Within an SDK directory, it will clean out stale entries within the local .speakeasy folder.

## Usage

```
speakeasy clean [flags]
```

### Options

```
      --global   clean out the root .speakeasy directory
  -h, --help     help for clean
```

### Options inherited from parent commands

```
      --logLevel string   the log level (available options: [info, warn, error]) (default "info")
```

### Parent Command

* [speakeasy](README.md)	 - The speakeasy cli tool provides access to the speakeasyapi.dev toolchain
