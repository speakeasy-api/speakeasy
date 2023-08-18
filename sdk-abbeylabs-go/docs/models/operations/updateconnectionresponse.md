# UpdateConnectionResponse


## Fields

| Field                                                   | Type                                                    | Required                                                | Description                                             |
| ------------------------------------------------------- | ------------------------------------------------------- | ------------------------------------------------------- | ------------------------------------------------------- |
| `Connection`                                            | [*shared.Connection](../../models/shared/connection.md) | :heavy_minus_sign:                                      | Success                                                 |
| `ContentType`                                           | *string*                                                | :heavy_check_mark:                                      | N/A                                                     |
| `Error`                                                 | [*shared.Error](../../models/shared/error.md)           | :heavy_minus_sign:                                      | Request Failed                                          |
| `StatusCode`                                            | *int*                                                   | :heavy_check_mark:                                      | N/A                                                     |
| `RawResponse`                                           | [*http.Response](https://pkg.go.dev/net/http#Response)  | :heavy_minus_sign:                                      | N/A                                                     |