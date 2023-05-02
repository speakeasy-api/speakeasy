package suggestions

import (
	"fmt"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"gopkg.in/yaml.v2"
	"regexp"
	"strconv"
	"strings"
)

func GetLineNumber(errStr string) (int, error) {
	lineStr := strings.Split(errStr, "[line ")[1]
	lineNumStr := strings.Split(lineStr, "]")[0]
	lineNum, err := strconv.Atoi(lineNumStr)
	if err != nil {
		return 0, err
	}
	return lineNum, nil
}

func EscapeString(input string) string {
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

func FormatYaml(input string) (string, error) {
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

func WaitForInput() {
	var input string
	for {
		fmt.Scan(&input)
		if strings.ToLower(input) == "yes" {
			// TODO: Update an actual openapi revision, maybe push to speakeasy registry
			fmt.Println(utils.Green("Suggestion Accepted"))
			fmt.Println() // extra space
			break
		}

		if strings.ToLower(input) == "no" {
			// TODO: Update an actual openapi revision, maybe push to speakeasy registry
			fmt.Println(utils.Red("Suggestion Rejected"))
			fmt.Println() // extra space
			break
		}
	}
}
