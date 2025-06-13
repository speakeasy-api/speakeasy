# TargetRunSummary


## Fields

| Field                                                       | Type                                                        | Required                                                    | Description                                                 |
| ----------------------------------------------------------- | ----------------------------------------------------------- | ----------------------------------------------------------- | ----------------------------------------------------------- |
| `TargetID`                                                  | *string*                                                    | :heavy_check_mark:                                          | Target ID in the workflow file                              |
| `SourceID`                                                  | *string*                                                    | :heavy_check_mark:                                          | Source ID in the workflow file                              |
| `OutputDirectory`                                           | *string*                                                    | :heavy_check_mark:                                          | Output directory for this target                            |
| `Language`                                                  | *string*                                                    | :heavy_check_mark:                                          | Language for this target                                    |
| `Readme`                                                    | [*components.FileData](../../models/components/filedata.md) | :heavy_minus_sign:                                          | N/A                                                         |
| `GenYaml`                                                   | [*components.FileData](../../models/components/filedata.md) | :heavy_minus_sign:                                          | N/A                                                         |