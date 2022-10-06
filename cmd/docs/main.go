package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/speakeasy-api/speakeasy/cmd"
	"github.com/spf13/cobra/doc"
)

var linkRegex = regexp.MustCompile(`\((.*?\.md)\)`)

func main() {
	root := cmd.GetRootCommand()

	root.DisableAutoGenTag = true

	if err := doc.GenMarkdownTree(root, "./docs"); err != nil {
		log.Fatal(err)
	}

	readmeData, err := os.ReadFile("./README.md")
	if err != nil {
		log.Fatal(err)
	}

	speakeasyData, err := os.ReadFile("./docs/speakeasy.md")
	if err != nil {
		log.Fatal(err)
	}

	speakeasyDoc := linkRegex.ReplaceAllStringFunc(string(speakeasyData), func(match string) string {
		return fmt.Sprintf("(docs/%s)", strings.Trim(match, "()"))
	})

	speakeasyDoc = strings.Replace(speakeasyDoc, "## speakeasy", "## Usage", 1)

	// boundary := `## Usage`

	// idx := strings.Index(string(readmeData), boundary)

	// readme := strings.TrimSuffix(string(readmeData), string(readmeData)[idx:])

	if err := os.WriteFile("./README.md", []byte(fmt.Sprintf("%s\n\n%s", string(readmeData), speakeasyDoc)), 0o644); err != nil {
		log.Fatal(err)
	}
}
