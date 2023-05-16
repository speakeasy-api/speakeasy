package suggestions

import (
	"fmt"
	"github.com/manifoldco/promptui"
	"gopkg.in/yaml.v2"
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
	// Unmarshal the YAML input into a generic interface{}
	var data interface{}
	if err := yaml.Unmarshal([]byte(input), &data); err != nil {
		return "", err
	}

	// Marshal the interface{} back to YAML with no extra spaces
	output, err := yaml.Marshal(data)
	if err != nil {
		return "", err
	}
	outputStr := strings.TrimSpace(string(output))

	return outputStr, nil
}

func acceptSuggestion() bool {
	templates := &promptui.SelectTemplates{
		Label:    "{{ . | cyan | bold }}",
		Active:   "🐝 {{ .Emoji | yellow }} {{ .Name | yellow | bold }}",
		Inactive: "   {{ .Emoji | white }} {{ .Name | white | bold }}",
		Selected: "> {{ .Emoji | green }} {{ .Name | green | bold }}",
	}

	prompt := promptui.Select{
		HideHelp: true,
		Label:    "Would you like to accept the suggestion?",
		Items: []map[string]interface{}{
			{
				"Emoji": "✅",
				"Name":  "Yes",
			},
			{
				"Emoji": "❌",
				"Name":  "No",
			},
		},
		Templates: templates,
		Size:      2,
	}

	index, _, err := prompt.Run()

	if err != nil {
		return false
	}

	return index == 0
}

func detectFileType(filename string) string {
	ext := filepath.Ext(filename)

	switch ext {
	case ".yaml", ".yml":
		return "text/yaml"
	case ".json":
		return "application/json"
	default:
		return "application/octet-stream"
	}
}

func FindSuggestion(err error, token string) {
	errString := err.Error()
	lineNumber, lineNumberErr := getLineNumber(errString)
	if lineNumberErr == nil {
		fmt.Println() // extra line for spacing
		fmt.Println(promptui.Styler(promptui.FGBold)("Asking for a Suggestion!"))
		suggestion, suggestionErr := Suggestion(token, errString, lineNumber)
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

			fix, yamlErr := formatYaml(escapeString(finalSplit[0][2:]))
			explanation := strings.TrimSpace(fmt.Sprintf("%s", escapeString(finalSplit[1][2:len(finalSplit[1])-1])))
			if yamlErr == nil {
				fmt.Println(promptui.Styler(promptui.FGGreen, promptui.FGBold)("Suggested Fix:"))
				fmt.Println(promptui.Styler(promptui.FGGreen, promptui.FGItalic)(fix))
				fmt.Println() // extra line for spacing
				fmt.Println(promptui.Styler(promptui.FGYellow, promptui.FGBold)("Explanation:"))
				fmt.Println(promptui.Styler(promptui.FGYellow, promptui.FGItalic)(explanation))
				fmt.Println() // extra line for spacing
				if acceptSuggestion() {
					// TODO: Best effort attempt to edit the OpenAPI File in place
					fmt.Println(promptui.Styler(promptui.FGGreen, promptui.FGBold)("Suggestion Accepted"))
					fmt.Println() // extra space
				} else {
					fmt.Println(promptui.Styler(promptui.FGRed, promptui.FGBold)("Suggestion Declined"))
					fmt.Println() // extra space
				}
			}
		} else {
			fmt.Println(promptui.Styler(promptui.FGRed, promptui.FGBold)("No Suggestion Found"))
		}
	}
}
