# Diagnostic


## Fields

| Field                                 | Type                                  | Required                              | Description                           |
| ------------------------------------- | ------------------------------------- | ------------------------------------- | ------------------------------------- |
| `Message`                             | *string*                              | :heavy_check_mark:                    | Message describing the issue          |
| `Severity`                            | *string*                              | :heavy_check_mark:                    | Severity                              |
| `Line`                                | **int64*                              | :heavy_minus_sign:                    | Line number                           |
| `Path`                                | []*string*                            | :heavy_minus_sign:                    | Schema path to the issue              |
| `Type`                                | *string*                              | :heavy_check_mark:                    | Issue type                            |
| `HelpMessage`                         | **string*                             | :heavy_minus_sign:                    | Help message for how to fix the issue |