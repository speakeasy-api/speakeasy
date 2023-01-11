package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/speakeasy-api/speakeasy/cmd"
	"github.com/speakeasy-api/speakeasy/internal/docs"
)

var linkRegex = regexp.MustCompile(`\((.*?\.md)\)`)

func main() {
	cmd.Init()

	root := cmd.GetRootCommand()

	root.DisableAutoGenTag = true

	docsDir := "./docs"

	if err := os.RemoveAll(docsDir); err != nil {
		log.Fatal(err)
	}

	if err := docs.GenerateDocs(root, docsDir); err != nil {
		log.Fatal(err)
	}

	readmeData, err := os.ReadFile("./README.md")
	if err != nil {
		log.Fatal(err)
	}

	readme, _, _ := strings.Cut(string(readmeData), "## CLI")

	speakeasyData, err := os.ReadFile("./docs/README.md")
	if err != nil {
		log.Fatal(err)
	}

	speakeasyDoc := linkRegex.ReplaceAllStringFunc(string(speakeasyData), func(match string) string {
		return fmt.Sprintf("(docs/%s)", strings.Trim(match, "()"))
	})

	speakeasyDoc = strings.ReplaceAll(speakeasyDoc, "## ", "### ")
	speakeasyDoc = strings.Replace(speakeasyDoc, "# speakeasy", "## CLI", 1)

	if err := os.WriteFile("./README.md", []byte(fmt.Sprintf("%s%s", readme, speakeasyDoc)), os.ModePerm); err != nil {
		log.Fatal(err)
	}
}
