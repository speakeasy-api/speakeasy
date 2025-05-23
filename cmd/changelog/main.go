package main

import (
	"flag"
	"log"

	"github.com/speakeasy-api/speakeasy/internal/changelog"
)

func main() {
	var input, output string

	flag.StringVar(&input, "input", "RELEASE_NOTES.md", "Path to the generated release notes")
	flag.StringVar(&output, "out-dir", "./marketing-site/src/content/changelog", "Directory to write .mdx files to")
	flag.Parse()

	if err := changelog.Generate(input, output); err != nil {
		log.Fatal(err)
	}
}
