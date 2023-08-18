# Oauth2AuthResponse


## Fields

| Field                                                          | Type                                                           | Required                                                       | Description                                                    |
| -------------------------------------------------------------- | -------------------------------------------------------------- | -------------------------------------------------------------- | -------------------------------------------------------------- |
| `ContentType`                                                  | *string*                                                       | :heavy_check_mark:                                             | N/A                                                            |
| `StatusCode`                                                   | *int*                                                          | :heavy_check_mark:                                             | N/A                                                            |
| `RawResponse`                                                  | [*http.Response](https://pkg.go.dev/net/http#Response)         | :heavy_minus_sign:                                             | N/A                                                            |
| `Token`                                                        | [*Oauth2AuthToken](../../models/operations/oauth2authtoken.md) | :heavy_minus_sign:                                             | Successful authentication.                                     |