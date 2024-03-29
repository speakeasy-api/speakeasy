package log

import (
	"context"
	"encoding/json"
	"github.com/spf13/cobra"
	"reflect"
)

func PrintArray[K any](cmd *cobra.Command, arr []K, fieldNameReplacements map[string]string) {
	printJson, _ := cmd.Flags().GetBool("json")

	if printJson {
		data, _ := json.Marshal(arr)
		From(cmd.Context()).Printf(string(data))
	} else {
		PrettyPrintArray(cmd.Context(), arr, fieldNameReplacements)
	}
}

func PrintValue(cmd *cobra.Command, value interface{}, fieldNameReplacements map[string]string) {
	printJson, _ := cmd.Flags().GetBool("json")
	l := From(cmd.Context())

	if printJson {
		data, _ := json.Marshal(value)
		l.Printf(string(data))
	} else {
		l.Printf("--------------------------------------")
		PrettyPrint(cmd.Context(), value, fieldNameReplacements)
	}
}

func PrettyPrintArray[K any](ctx context.Context, arr []K, fieldNameReplacements map[string]string) {
	l := From(ctx)

	if len(arr) == 0 {
		l.Printf("NO RESULTS")
		return
	}

	l.Printf("--------------------------------------")
	for _, item := range arr {
		PrettyPrint(ctx, item, fieldNameReplacements)
		l.Printf("--------------------------------------")
	}
}

func PrettyPrint(ctx context.Context, value interface{}, fieldNameReplacements map[string]string) {
	l := From(ctx)

	refVal := reflect.ValueOf(value)

	if refVal.Kind() == reflect.Ptr {
		refVal = refVal.Elem()
	}

	if refVal.Kind() != reflect.Struct {
		l.PrintlnUnstyled(value)
	}

	for i := 0; i < refVal.NumField(); i++ {
		field := refVal.Type().Field(i)
		fieldName := field.Name
		val := refVal.Field(i)

		if field.Type.Kind() == reflect.Ptr && !val.IsNil() {
			val = val.Elem()
		}

		value := val.Interface()

		if val.Type().Kind() == reflect.Struct || val.Type().Kind() == reflect.Map || val.Type().Kind() == reflect.Slice || val.Type().Kind() == reflect.Array {
			data, _ := json.Marshal(value)
			value = string(data)
		}

		if fieldNameReplacements != nil {
			if replacement, ok := fieldNameReplacements[fieldName]; ok {
				fieldName = replacement
			}
		}

		l.Printf("%s: %v", fieldName, value)
	}
}
