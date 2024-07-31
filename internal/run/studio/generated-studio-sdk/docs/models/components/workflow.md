# Workflow


## Fields

| Field                                                             | Type                                                              | Required                                                          | Description                                                       |
| ----------------------------------------------------------------- | ----------------------------------------------------------------- | ----------------------------------------------------------------- | ----------------------------------------------------------------- |
| `Version`                                                         | *string*                                                          | :heavy_check_mark:                                                | Workflow version                                                  |
| `SpeakeasyVersion`                                                | *string*                                                          | :heavy_check_mark:                                                | Speakeasy version                                                 |
| `Sources`                                                         | map[string][components.Source](../../models/components/source.md) | :heavy_check_mark:                                                | Map of sources                                                    |
| `Targets`                                                         | map[string][components.Target](../../models/components/target.md) | :heavy_check_mark:                                                | Map of targets                                                    |