# BigInt

`types.BigInt` is a wrapper around big.Int that allows for JSON marshaling a big integer as a string.

## Usage

```go
b := types.MustBigIntFromString("1") // Returns nil if the string is not a valid integer
```