package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"reflect"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	Green  = color.New(color.FgGreen).SprintFunc()
	Red    = color.New(color.FgRed).SprintFunc()
	Yellow = color.New(color.FgYellow).SprintFunc()
	Blue   = color.New(color.FgBlue).SprintFunc()

	BackgroundYellow       = color.New(color.BgHiYellow).SprintFunc()
	BackgroundYellowBoldFG = color.New(color.BgHiYellow).Add(color.FgBlack).Add(color.Bold).SprintFunc()
)

func PrintArray[K any](cmd *cobra.Command, arr []K, fieldNameReplacements map[string]string) {
	printJson, _ := cmd.Flags().GetBool("json")

	if printJson {
		data, _ := json.Marshal(arr)
		fmt.Println(string(data))
	} else {
		PrettyPrintArray(arr, fieldNameReplacements)
	}
}

func PrintValue(cmd *cobra.Command, value interface{}, fieldNameReplacements map[string]string) {
	printJson, _ := cmd.Flags().GetBool("json")

	if printJson {
		data, _ := json.Marshal(value)
		fmt.Println(string(data))
	} else {
		fmt.Println("--------------------------------------")
		PrettyPrint(value, fieldNameReplacements)
	}
}

func PrettyPrintArray[K any](arr []K, fieldNameReplacements map[string]string) {
	if len(arr) == 0 {
		fmt.Println("NO RESULTS")
		return
	}

	fmt.Println("--------------------------------------")
	for _, item := range arr {
		PrettyPrint(item, fieldNameReplacements)
		fmt.Println("--------------------------------------")
	}
}

func PrettyPrint(value interface{}, fieldNameReplacements map[string]string) {
	refVal := reflect.ValueOf(value)

	if refVal.Kind() == reflect.Ptr {
		refVal = refVal.Elem()
	}

	if refVal.Kind() != reflect.Struct {
		fmt.Println(value)
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

		fmt.Printf("%s: %v\n", fieldName, value)
	}
}

func CreateDirectory(filename string) error {
	dir := path.Dir(filename)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err := os.MkdirAll(dir, os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil
}
