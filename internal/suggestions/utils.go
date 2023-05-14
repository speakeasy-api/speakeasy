package suggestions

import (
	"fmt"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"gopkg.in/yaml.v2"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func getLineNumber(errStr string) (int, error) {
	lineStr := strings.Split(errStr, "[line ")[1]
	lineNumStr := strings.Split(lineStr, "]")[0]
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

func waitForInput() {
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
		fmt.Println("Asking for a Suggestion")
		suggestion, suggestionErr := Suggestion(token, errString, lineNumber)
		if suggestionErr == nil && suggestion != "" && !strings.Contains(suggestion, "I cannot provide an answer") {
			fixSplit := strings.Split(suggestion, "Suggested Fix:")
			if len(fixSplit) < 2 {
				fmt.Println(utils.Yellow("No Suggestion Found"))
				return
			}
			split := strings.Split(fixSplit[1], "Explanation:")
			fix, yamlErr := formatYaml(escapeString(split[0][2:]))
			if yamlErr == nil {
				fmt.Println(utils.Green("Suggested Fix:"))
				fmt.Println(utils.Green(fix))
				fmt.Println() // extra line for spacing
				fmt.Println(utils.Yellow("Explanation:"))
				explanation := strings.TrimSpace(fmt.Sprintf("%s", escapeString(split[1][2:len(split[1])-1])))
				fmt.Println(utils.Yellow(fmt.Sprintf("%s", explanation)))
				fmt.Println() // extra line for spacing
				fmt.Println(fmt.Sprintf("Type %s and Enter to accept the suggestion, type %s and Enter to skip:", utils.Green("yes"), utils.Red("no")))
				waitForInput()
			}
		} else {
			fmt.Println(utils.Yellow("No Suggestion Found"))
		}
	}
}
