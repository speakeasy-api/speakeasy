---
name: fix-validation-errors-with-overlays
description: Use when you have lint errors but can't modify the source spec, or need to add missing descriptions/tags via overlay
---

# fix-validation-errors-with-overlays

## Overlay-Appropriate Fixes

| Issue | Overlay Solution |
|-------|------------------|
| Poor operation names | `x-speakeasy-name-override` |
| Missing descriptions | Add `summary` or `description` |
| Missing tags | Add `tags` array |
| Need operation grouping | `x-speakeasy-group` |
| Need retry config | `x-speakeasy-retries` |

## NOT Overlay-Appropriate

| Issue | Why |
|-------|-----|
| Invalid JSON/YAML | Syntax error in source |
| Missing required fields | Schema incomplete |
| Broken $ref | Source needs fixing |
| Wrong data types | API design issue |

## Quick Fix Workflow

```bash
# 1. Generate suggestions
speakeasy suggest operation-ids -s openapi.yaml -o fixes.yaml

# 2. Add to workflow
# Edit .speakeasy/workflow.yaml to include overlay

# 3. Regenerate
speakeasy run
```
