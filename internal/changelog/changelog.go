package changelog

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var headerPattern = regexp.MustCompile(`(?m)^## \[(\d+\.\d+\.\d+)\]\([^)]+\) \((\d{4}-\d{2}-\d{2})\)`)
var sectionPattern = regexp.MustCompile(`(?m)^### ([^#\n]+)\s*\n(.*?)(?:\n###|$)`)
var languagePattern = regexp.MustCompile(`\b(PHP|Ruby|Java|Python|TypeScript|JavaScript|Go|C#|Swift|Kotlin)\b`)
var urlPattern = regexp.MustCompile(`https://github\.com/speakeasy-api/cli/pull/`)

var sectionEmojis = map[string]string{
	"features":                 "✨",
	"bug fixes":                "🐛",
	"fixes":                    "🐛",
	"performance improvements": "⚡",
	"reverts":                  "⏪",
	"documentation":            "📚",
	"styles":                   "💄",
	"refactor":                 "♻️",
	"tests":                    "🧪",
	"build":                    "🔨",
	"ci":                       "👷",
	"chore":                    "🧹",
	"chores":                   "🧹",
	"improvement":              "🚀",
	"improvements":             "🚀",
	"breaking changes":         "💥",
	"deprecation":              "⚠️",
	"security":                 "🔒",
}

func Generate(inputFile, outputDir string) error {
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	content := string(data)
	entries := headerPattern.FindAllStringIndex(content, -1)

	if len(entries) == 0 {
		fmt.Println("No changelog entries found.")
		return nil
	}

	for i := 0; i < len(entries); i++ {
		start := entries[i][0]
		end := len(content)
		if i+1 < len(entries) {
			end = entries[i+1][0]
		}
		chunk := strings.TrimSpace(content[start:end])

		submatches := headerPattern.FindStringSubmatch(chunk)
		if len(submatches) != 3 {
			continue
		}

		version := submatches[1]
		date := submatches[2]
		body := strings.TrimSpace(chunk[len(submatches[0]):])
		body = strings.TrimSuffix(body, "---")
		body = strings.TrimSpace(body)

		if body == "" {
			fmt.Printf("Skipping v%s — no content.\n", version)
			continue
		}

		slug := "v" + strings.ReplaceAll(version, ".", "-")
		filePath := filepath.Join(outputDir, slug+".mdx")

		if _, err := os.Stat(filePath); err == nil {
			fmt.Printf("Skipping v%s — file already exists at %s\n", version, filePath)
			continue
		}

		processedBody := addEmojisToSections(body)

		content := strings.Builder{}
		content.WriteString("---\n")
		content.WriteString(fmt.Sprintf("title: \"v%s\"\n", version))
		content.WriteString(fmt.Sprintf("date: \"%s\"\n", date))
		content.WriteString("# IMPORTANT: Add tags and authors like so:\n")
		content.WriteString("# tags:\n")
		content.WriteString("#   - \"Python\"\n")
		content.WriteString("#   - \"Ruby\"\n")
		content.WriteString("# authors:\n")
		content.WriteString("#   - name: Sagar Batchu\n")
		content.WriteString("#   - image_url: \"/assets/author-headshots/sagar.jpeg\"\n")
		content.WriteString("---\n\n")
		content.WriteString(processedBody + "\n")

		if err := os.WriteFile(filePath, []byte(content.String()), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", filePath, err)
		}

		fmt.Printf("Created changelog file: %s\n", filePath)
	}

	return nil
}

func addEmojisToSections(body string) string {
	lines := strings.Split(body, "\n")
	var result []string
	sectionCount := 0

	// Count sections
	for _, line := range lines {
		if strings.HasPrefix(line, "### ") {
			sectionCount++
		}
	}

	foundFirstSection := false
	for _, line := range lines {
		if strings.HasPrefix(line, "### ") {
			sectionName := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "### ")))
			if emoji, exists := sectionEmojis[sectionName]; exists {
				line = "### " + emoji + " " + strings.TrimSpace(strings.TrimPrefix(line, "### "))
			}

			// If this is the second section and we have multiple sections, add HR before it
			if foundFirstSection && sectionCount > 1 {
				result = append(result, "", "<hr />", "")
			}
			foundFirstSection = true
		}
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}
