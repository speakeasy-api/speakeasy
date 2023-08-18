# ListTasksResponse


## Fields

| Field                                                  | Type                                                   | Required                                               | Description                                            |
| ------------------------------------------------------ | ------------------------------------------------------ | ------------------------------------------------------ | ------------------------------------------------------ |
| `ContentType`                                          | *string*                                               | :heavy_check_mark:                                     | N/A                                                    |
| `Error`                                                | [*shared.Error](../../models/shared/error.md)          | :heavy_minus_sign:                                     | Authentication Failed                                  |
| `StatusCode`                                           | *int*                                                  | :heavy_check_mark:                                     | N/A                                                    |
| `RawResponse`                                          | [*http.Response](https://pkg.go.dev/net/http#Response) | :heavy_minus_sign:                                     | N/A                                                    |
| `Tasks`                                                | [][shared.Task](../../models/shared/task.md)           | :heavy_minus_sign:                                     | Success                                                |