package suggestions

import (
	"context"
	"fmt"
	"os"

	"github.com/speakeasy-api/speakeasy/internal/ci/environment"
	"github.com/speakeasy-api/speakeasy/internal/ci/git"
	"github.com/speakeasy-api/speakeasy/internal/ci/logging"
	"github.com/speakeasy-api/speakeasy/internal/suggest"
	"github.com/speakeasy-api/speakeasy/internal/utils"

	"regexp"
	"strconv"
	"strings"
)

var (
	fileNameRegex      = regexp.MustCompile(`^(.*?(\.yaml|\.yml|\.json))`)
	outputFileRegex    = regexp.MustCompile(`Suggestions applied and written to (.+)`)
	validationErrRegex = regexp.MustCompile(`(validation (hint|warn|error):)\s+\[line (\d+)\]\s+(.*)$`)
)

type prCommentsInfo struct {
	suggestions  []string
	explanations []string
	errs         []string
	lineNums     []int
}

func Suggest(ctx context.Context, docPath, maxSuggestions string) (string, error) {
	outputPath := environment.GetOpenAPIDocOutput()

	outFile, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	yamlOut := utils.HasYAMLExt(outputPath)
	if err := suggest.SuggestAndWrite(ctx, "method-names", docPath, false, yamlOut, outFile); err != nil {
		return "", err
	}

	return "", nil
}

func WriteSuggestions(g *git.Git, prNumber int, out string) error {
	commentsInfo, fileName := parseSuggestOutput(out)

	for i := 0; i < len(commentsInfo.lineNums); i++ {
		if commentsInfo.lineNums[i] != 0 {
			comment := formatComment(commentsInfo.errs[i], commentsInfo.suggestions[i], commentsInfo.explanations[i], 0, i+1)
			if err := g.WritePRComment(prNumber, fileName, comment, commentsInfo.lineNums[i]); err != nil {
				logging.Info("failed to write PR comment: %s", err.Error())
				commentsInfo.lineNums[i] = 0
			}
		}
	}

	body := formatBody(commentsInfo)
	if body != "" {
		// Writes suggestions and explanations with line number 0 PR body
		if err := g.WritePRBody(prNumber, body); err != nil {
			return fmt.Errorf("error writing PR body: %w", err)
		}
	}

	return nil
}

func formatComment(err, suggestion, explanation string, lineNum, index int) string {
	var commentParts []string
	commentParts = append(commentParts, fmt.Sprintf("**Error %d**: %s", index, err))

	suggestion = fmt.Sprintf("**Suggestion %d**: %s", index, suggestion)
	if lineNum != 0 {
		suggestion = fmt.Sprintf("*Applied around line %d*\n%s", lineNum, suggestion)
	}
	commentParts = append(commentParts, suggestion)
	commentParts = append(commentParts, fmt.Sprintf("**Explanation %d**: %s", index, explanation))
	return strings.Join(commentParts, "\n\n")
}

func formatBody(info prCommentsInfo) string {
	var bodyParts []string
	for i := 0; i < len(info.lineNums); i++ {
		if info.lineNums[i] == 0 {
			bodyParts = append(bodyParts, formatComment(info.errs[i], info.suggestions[i], info.explanations[i], info.lineNums[i], i+1))
		}
	}
	return strings.Join(bodyParts, "\n\n")
}

func parseSuggestOutput(out string) (prCommentsInfo, string) {
	var info prCommentsInfo
	var lineNum int
	var err error
	lines := strings.Split(out, "\n")
	suggestion, explanation, validationErr, fileName := "", "", "", ""
	isSuggestion, isExplanation := false, false

	for _, line := range lines {
		validationErrMatch := validationErrRegex.FindStringSubmatch(line)
		if len(validationErrMatch) == 5 {
			lineNum, err = strconv.Atoi(validationErrMatch[3])
			if err != nil {
				// line number 0 indicates adding this validation error, suggestion, and explanation to PR body
				lineNum = 0
			}
			validationErr = validationErrMatch[4]
			continue
		}

		if strings.Contains(line, "Suggestion:") {
			isSuggestion = true
			if strings.TrimSpace(suggestion) != "" {
				info.suggestions = append(info.suggestions, suggestion)
			}
			if strings.TrimSpace(validationErr) != "" {
				info.errs = append(info.errs, validationErr)
			}
			info.lineNums = append(info.lineNums, lineNum)

			suggestion, validationErr = "", ""
			lineNum = 0
			continue
		}

		if strings.Contains(line, "Explanation:") {
			isSuggestion = false
			isExplanation = true
			if strings.TrimSpace(explanation) != "" {
				info.explanations = append(info.explanations, explanation)
			}
			explanation = ""
			continue
		}

		outputFileMatch := outputFileRegex.FindStringSubmatch(line)
		if len(outputFileMatch) == 2 {
			// Remove leading ./ and anything trailing after file suffix
			outputFileName := strings.Replace(outputFileMatch[1], "./", "", 1)
			fileNameMatch := fileNameRegex.FindStringSubmatch(outputFileName)
			if len(fileNameMatch) > 0 {
				fileName = fileNameMatch[0]
			}
			continue
		}

		if strings.TrimSpace(line) == "" {
			isSuggestion, isExplanation = false, false
		}

		if isSuggestion {
			suggestion += line
		}
		if isExplanation {
			explanation += line
		}
	}

	if strings.TrimSpace(suggestion) != "" {
		info.suggestions = append(info.suggestions, suggestion)
	}
	if strings.TrimSpace(explanation) != "" {
		info.explanations = append(info.explanations, explanation)
	}

	return info, fileName
}
