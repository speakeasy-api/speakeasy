# SourceResponse


## Fields

| Field                                                            | Type                                                             | Required                                                         | Description                                                      |
| ---------------------------------------------------------------- | ---------------------------------------------------------------- | ---------------------------------------------------------------- | ---------------------------------------------------------------- |
| `SourceID`                                                       | *string*                                                         | :heavy_check_mark:                                               | Source ID in the workflow file                                   |
| `Input`                                                          | *string*                                                         | :heavy_check_mark:                                               | The merged input specs for the source                            |
| `Overlay`                                                        | *string*                                                         | :heavy_check_mark:                                               | Studio modifications overlay contents (could be empty string)    |
| `OverlayPath`                                                    | *string*                                                         | :heavy_check_mark:                                               | Studio modifications overlay path                                |
| `Output`                                                         | *string*                                                         | :heavy_check_mark:                                               | Result of running the source in the workflow                     |
| `Diagnosis`                                                      | [][components.Diagnostic](../../models/components/diagnostic.md) | :heavy_check_mark:                                               | List of validation errors                                        |