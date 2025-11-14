package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/speakeasy-api/speakeasy/cmd"
	"github.com/speakeasy-api/speakeasy/internal/docs"
)

var linkRegex = regexp.MustCompile(`\((.*?\.md)\)`)

func main() {
	outDir := flag.String("out-dir", "./docs", "The directory to output the docs to")
	flag.Parse()

	cmd.Init("", "")

	root := cmd.GetRootCommand()

	root.DisableAutoGenTag = true

	exclusionList := []string{
		filepath.Join(*outDir, "getting-started.mdx"),
		filepath.Join(*outDir, "_meta.tsx"),
		filepath.Join(*outDir, "mise-toolkit.mdx"),
	}

	if _, err := removeDocs(*outDir, exclusionList); err != nil {
		log.Fatal(err)
	}

	if err := docs.GenerateDocs(root, *outDir); err != nil {
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
			childEmpty, err := removeDocs(filepath.Join(outDir, item.Name()), exclusionList)
			if err != nil {
				return false, err
			}

			if childEmpty {
				if err := os.Remove(filepath.Join(outDir, item.Name())); err != nil {
					return false, err
				}
			} else {
				empty = false
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
