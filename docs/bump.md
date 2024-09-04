# bump  
`speakeasy bump`  


Bumps the version of a Speakeasy Generation Target  

## Details

# Bump

Bumps the version of a Speakeasy Generation Target, run within the target's directory. Allows the bumping of patch, minor, and major versions or setting to a specific version.

Examples:

- speakeasy bump patch - Bumps the target's version by one patch version
- speakeasy bump -v 1.2.3 - Sets the target's version to 1.2.3
- speakeasy bump major -t typescript - Bumps the typescript target's version by one major version
- speakeasy bump graduate - Current version 1.2.3-alpha.1 sets the target's version to 1.2.3


## Usage

```
speakeasy bump [patch|minor|major|graduate] [flags]
```

### Examples

```


const version = require("@speakeasy/sdk-typescript").version;
console.log(version);

```

### Options

```
  -h, --help             help for bump
  -t, --target string    The target to bump the version of, if more than one target is found in the gen.yaml
  -v, --version string   The version to bump to, if you want to specify a specific version.
```

### Options inherited from parent commands

```
      --logLevel string   the log level (available options: [info, warn, error]) (default "info")
```

### Parent Command

* [speakeasy](README.md)	 - The Speakeasy CLI tool provides access to the Speakeasy.com platform
