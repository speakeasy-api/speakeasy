# RunResponse


## Fields

| Field                                   | Type                                    | Required                                | Description                             |
| --------------------------------------- | --------------------------------------- | --------------------------------------- | --------------------------------------- |
| `Errors`                                | []*string*                              | :heavy_minus_sign:                      | List of errors                          |
| `Warnings`                              | []*string*                              | :heavy_minus_sign:                      | List of warnings                        |
| `Info`                                  | []*string*                              | :heavy_minus_sign:                      | List of informational messages          |
| `LintingReportLink`                     | **string*                               | :heavy_minus_sign:                      | Link to the linting report              |
| `LintingErrorCount`                     | **int64*                                | :heavy_minus_sign:                      | Count of linting errors                 |
| `LintingWarningCount`                   | **int64*                                | :heavy_minus_sign:                      | Count of linting warnings               |
| `LintingInfoCount`                      | **int64*                                | :heavy_minus_sign:                      | Count of linting informational messages |