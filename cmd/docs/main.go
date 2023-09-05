package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/speakeasy-api/speakeasy/cmd"
	"github.com/speakeasy-api/speakeasy/internal/docs"
	"golang.org/x/exp/slices"
)

var linkRegex = regexp.MustCompile(`\((.*?\.md)\)`)

func main() {
	outDir := flag.String("out-dir", "./docs", "The directory to output the docs to")
	docSite := flag.Bool("doc-site", false, "Whether to generate docs for the doc site")
	flag.Parse()

	cmd.Init("", "")

	root := cmd.GetRootCommand()

	root.DisableAutoGenTag = true

	exclusionList := []string{
		filepath.Join(*outDir, "01-getting-started.md"),
		filepath.Join(*outDir, "_category_.json"),
	}

	if _, err := removeDocs(*outDir, exclusionList); err != nil {
		log.Fatal(err)
	}

	if err := docs.GenerateDocs(root, *outDir, *docSite); err != nil {
		log.Fatal(err)
	}

	if *docSite {
		return
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

	if err := os.WriteFile("./README.md", []byte(fmt.Sprintf("%s%s", readme, speakeasyDoc)), 0o644); err != nil {
		log.Fatal(err)
	}
}

func removeDocs(outDir string, exclusionList []string) (bool, error) {
	items, err := os.ReadDir(outDir)
	if err != nil {
		return false, err
	}

	empty := true

	for _, item := range items {
		if item.IsDir() {
			empty, err := removeDocs(filepath.Join(outDir, item.Name()), exclusionList)
			if err != nil {
				return false, err
			}

			if empty {
				if err := os.Remove(filepath.Join(outDir, item.Name())); err != nil {
					return false, err
				}
			}
		} else {
			if slices.Contains(exclusionList, filepath.Join(outDir, item.Name())) {
				empty = false
				continue
			}

			if err := os.Remove(filepath.Join(outDir, item.Name())); err != nil {
				return false, err
			}
		}
	}

	return empty, nil
}
