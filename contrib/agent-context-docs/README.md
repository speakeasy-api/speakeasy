# Agent Context Documentation

This directory contains documentation content intended for the [speakeasy-agent-mode-content](https://github.com/speakeasy-api/speakeasy-agent-mode-content) repository.

## Files

- `server-sent-events.md` - Documentation for modeling SSE in OpenAPI for Speakeasy SDK generation

## How to Use

Copy the markdown files from this directory to the appropriate location in the `speakeasy-agent-mode-content` repository to make them available through `speakeasy agent context`.

## Required Updates to Original Documentation

### Add C# to Supported Languages for SSE

The original documentation at https://www.speakeasy.com/docs/customize/runtime/server-sent-events should be updated to include **C#** in the list of supported languages for SSE.

**Current supported languages (as documented):**
- TypeScript
- Python
- Java

**Should include:**
- TypeScript
- Python
- Java
- **C#** (NEW)
- Go

The October 2025 release blog post at https://www.speakeasy.com/blog/release-sse-improvements confirms C# support was added, but the main documentation page may not reflect this.

### Suggested Documentation Change

In the "Supported Languages" or similar section, update the list to:

```
SSE streaming is supported in:
- TypeScript
- Python
- Java
- C#
- Go
```
