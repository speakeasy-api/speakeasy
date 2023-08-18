<!-- Start SDK Example Usage -->


```go
package main

import(
	"context"
	"log"
	"openapi"
	"openapi/pkg/models/shared"
)

func main() {
    s := sdk.New(
        sdk.WithSecurity(shared.Security{
            APIKeyAuth: sdk.String("Token YOUR_API_KEY"),
        }),
        sdk.WithGlobalPathParam(100),
        sdk.WithGlobalQueryParam("some example global query param"),
    )

    ctx := context.Background()
    res, err := s.Generation.GlobalNameOverridden(ctx)
    if err != nil {
        log.Fatal(err)
    }

    if res.GetGlobalNameOverride200ApplicationJSONObject != nil {
        // handle response
    }
}
```


## Second
Do this second
```go
package main

import(
	"context"
	"log"
	"openapi"
	"openapi/pkg/models/operations"
	"openapi/pkg/models/shared"
	"math/big"
	"openapi/pkg/types"
)

func main() {
    s := sdk.New(
        sdk.WithGlobalPathParam(100),
        sdk.WithGlobalQueryParam("some example global query param"),
    )

    ctx := context.Background()
    res, err := s.Generation.UsageExamplePost(ctx, operations.UsageExamplePostRequest{
        RequestBody: &operations.UsageExamplePostRequestBody{
            Email: sdk.String("Larue_Rau85@yahoo.com"),
            FormatEmail: sdk.String("Roselyn_Kassulke@yahoo.com"),
            FormatURI: sdk.String("http://innocent-effect.org"),
            FormatUUID: sdk.String("0f467cc8-796e-4d15-9a05-dfc2ddf7cc78"),
            Hostname: sdk.String("soulful-poppy.com"),
            Ipv4: sdk.String("184.163.148.36"),
            Ipv6: sdk.String("8fc8:1674:2cb7:3920:5929:396f:ea75:96eb"),
            SimpleObject: &shared.SimpleObject{
                Any: "architecto",
                Bigint: big.NewInt(60225),
                BigintStr: types.MustBigIntFromString("969810"),
                Bool: true,
                BoolOpt: sdk.Bool(true),
                Date: types.MustDateFromString("2020-01-01"),
                DateTime: types.MustTimeFromString("2020-01-01T00:00:00Z"),
                Enum: shared.EnumTwo,
                Float32: 2.2222222,
                Int: 999999,
                Int32: 1,
                Int32Enum: shared.SimpleObjectInt32EnumOneHundredAndEightyOne,
                IntEnum: shared.SimpleObjectIntEnumSecond,
                IntOptNull: sdk.Int64(999999),
                Num: 1.1,
                NumOptNull: sdk.Float64(1.1),
                Str: "example",
                StrOpt: sdk.String("optional example"),
            },
            Unknown: sdk.String("laborum"),
            URI: sdk.String("http://doting-footage.com"),
            UUID: sdk.String("c5955907-aff1-4a3a-afa9-467739251aa5"),
        },
        BoolParameter: false,
        DateParameter: types.MustDateFromString("2020-01-01"),
        DateTimeParameter: types.MustTimeFromString("2020-01-01T00:00:00Z"),
        DoubleParameter: 2.2222222,
        EnumParameter: operations.UsageExamplePostEnumParameterValue3,
        FalseyNumberParameter: 0,
        FloatParameter: 1.1,
        Int64Parameter: 111111,
        IntParameter: 1,
        OptEnumParameter: operations.UsageExamplePostOptEnumParameterValue3.ToPointer(),
        StrParameter: "example 1",
    }, operations.UsageExamplePostSecurity{
        Password: "YOUR_PASSWORD",
        Username: "YOUR_USERNAME",
    })
    if err != nil {
        log.Fatal(err)
    }

    if res.UsageExamplePost200ApplicationJSONObject != nil {
        // handle response
    }
}
```
<!-- End SDK Example Usage -->