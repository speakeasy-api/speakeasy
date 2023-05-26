package suggestions

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/manifoldco/promptui"
	"gopkg.in/yaml.v2"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func getLineNumber(errStr string) (int, error) {
	lineStr := strings.Split(errStr, "[line ")
	if len(lineStr) < 2 {
		return 0, fmt.Errorf("line number cannot be found in err %s", errStr)
	}

	lineNumStr := strings.Split(lineStr[1], "]")[0]
	lineNum, err := strconv.Atoi(lineNumStr)
	if err != nil {
		return 0, err
	}

	return lineNum, nil
}

func escapeString(input string) string {
	re := regexp.MustCompile(`\\([abfnrtv\\'"])`)

	// Replace escape sequences with their unescaped counterparts
	output := re.ReplaceAllStringFunc(input, func(match string) string {
		switch match {
		case `\a`:
			return "\a"
		case `\b`:
			return "\b"
		case `\f`:
			return "\f"
		case `\n`:
			return "\n"
		case `\r`:
			return "\r"
		case `\t`:
			return "\t"
		case `\v`:
			return "\v"
		case `\\`:
			return `\`
		case `\'`:
			return `'`
		case `\"`:
			return `"`
		default:
			// Should never happen, but just in case
			return match
		}
	})

	return output
}

func formatYaml(input string) (string, error) {
	var data interface{}
	if err := yaml.Unmarshal([]byte(input), &data); err != nil {
		return "", err
	}

	output, err := yaml.Marshal(data)
	if err != nil {
		return "", err
	}
	outputStr := strings.TrimSpace(string(output))

	return outputStr, nil
}

func removeTrailingComma(input string) string {
	trimmed := strings.TrimRight(input, "\n \t\r")
	if strings.HasSuffix(trimmed, ",") {
		trimmed = trimmed[:len(trimmed)-1]
	}
	return trimmed
}

func formatJSON(input string) (string, error) {
	var data interface{}
	jsonString := fmt.Sprintf(`{%s}`, removeTrailingComma(input))
	if err := json.Unmarshal([]byte(jsonString), &data); err != nil {
		return "", err
	}

	output, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	outputStr := strings.TrimSpace(string(output))

	return outputStr, nil
}

func nextSuggestion() {
	fmt.Print(promptui.Styler(promptui.FGCyan, promptui.FGBold)("🐝 Press 'Enter' to continue"))
	bufio.NewReader(os.Stdin).ReadBytes('\n')
	fmt.Println() // extra line for spacing
}

func detectFileType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".yaml", ".yml":
		return "text/yaml"
	case ".json":
		return "application/json"
	default:
		return "application/octet-stream"
	}
}

func FindSuggestion(err error, token string, fileType string) {
	errString := err.Error()
	lineNumber, lineNumberErr := getLineNumber(errString)
	if lineNumberErr == nil {
		fmt.Println() // extra line for spacing
		fmt.Println(promptui.Styler(promptui.FGBold)("Asking for a Suggestion!"))
		suggestion, suggestionErr := Suggestion(token, errString, lineNumber, fileType)
		if suggestionErr == nil && suggestion != "" && !strings.Contains(suggestion, "I cannot provide an answer") {
			fixSplit := strings.Split(suggestion, "Suggested Fix:")
			if len(fixSplit) < 2 {
				fmt.Println(promptui.Styler(promptui.FGRed, promptui.FGBold)("No Suggestion Found"))
				return
			}

			finalSplit := strings.Split(fixSplit[1], "Explanation:")
			if len(finalSplit) < 2 || len(finalSplit[0]) < 3 || len(finalSplit[1]) < 3 {
				fmt.Println(promptui.Styler(promptui.FGRed, promptui.FGBold)("No Suggestion Found"))
				return
			}

			var fix string
			var marshalErr error
			if fileType == "json" {
				fix, marshalErr = formatJSON(escapeString(finalSplit[0][2:]))
			} else {
				fix, marshalErr = formatYaml(escapeString(finalSplit[0][2:]))
			}

			explanation := strings.TrimSpace(fmt.Sprintf("%s", escapeString(finalSplit[1][2:len(finalSplit[1])-1])))
			if marshalErr == nil {
				fmt.Println(promptui.Styler(promptui.FGGreen, promptui.FGBold)("Suggested Fix:"))
				fmt.Println(promptui.Styler(promptui.FGGreen, promptui.FGItalic)(fix))
				fmt.Println() // extra line for spacing
				fmt.Println(promptui.Styler(promptui.FGYellow, promptui.FGBold)("Explanation:"))
				fmt.Println(promptui.Styler(promptui.FGYellow, promptui.FGItalic)(explanation))
				fmt.Println() // extra line for spacing
				nextSuggestion()
				return
			}
		} else {
			if suggestionErr != nil {
				if strings.Contains(suggestionErr.Error(), "401") || strings.Contains(suggestionErr.Error(), "403") {
					fmt.Println(promptui.Styler(promptui.FGRed, promptui.FGBold)(fmt.Sprintf("No Suggestion Found: %s", suggestionErr.Error())))
					return
				}
			}
		}
	}
	fmt.Println(promptui.Styler(promptui.FGRed, promptui.FGBold)("No Suggestion Found"))
}
