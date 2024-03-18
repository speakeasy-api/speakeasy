package main

import (
	"fmt"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/validation"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
	"os"
	"strings"
)

var (
	version      = "0.0.1"
	artifactArch = "linux_amd64"
)

func main() {
	fileBytes, err := os.ReadFile("out.txt")
	if err != nil {
		println(err)
	}

	configs := strings.Split(string(fileBytes), "configVersion")

	count := 0

	for _, c := range configs {
		if count >= 15 {
			return
		}
		if strings.TrimSpace(c) == "" {
			continue
		}
		c = "configVersion" + c

		cfgMap := map[string]any{}
		if err := yaml.Unmarshal([]byte(c), &cfgMap); err != nil {
			println("failed with config:\n", c)
			fmt.Printf("could not unmarshal gen.yaml: %s", err.Error())
			return
		}

		for target, cfg := range cfgMap {
			_, err := generate.GetTargetFromTargetString(target)
			if err != nil {
				continue
			}

			subCfg, ok := cfg.(map[string]any)
			if !ok {
				return
			}
			//if err := yaml.Unmarshal(cfg.([]byte), &subCfg); err != nil {
			//	fmt.Printf("could not unmarshal SUB gen.yaml: %s", err.Error())
			//	return
			//}

			if target == "javav2" {
				println(c)
			}

			errs := validation.ValidateTarget(target, subCfg, false)

			if len(errs) > 0 {
				if len(errs) == 1 && slices.ContainsFunc(allowedStrings, func(s string) bool { return strings.Contains(errs[0].Error(), s) }) {
					continue
				}

				errsS := "\t"
				for _, err := range errs {
					errsS += err.Error() + "\n\t"
				}

				//fmt.Printf("\n\n\ngot %d errs validating target %s.\n\nErrs:\n%s\n\n%s", len(errs), target, errsS, c)
				count++
			}
		}
	}
}

var allowedStrings = []string{
	"rootProject.name",
	"groupID",
	"https://getcomposer.org/doc/04-schema.md#name",
	"The go module package name",
}

func removeLeadingSpaces() {
	fileBytes, err := os.ReadFile("out.txt")

	if err != nil {
		return
	}

	lines := strings.Split(string(fileBytes), "\n")
	for i, line := range lines {
		if len(line) > 0 {
			lines[i] = line[1:]
		}
	}

	os.WriteFile("out.txt", []byte(strings.Join(lines, "\n")), 0644)
}

func trimDbData() {
	fileBytes, err := os.ReadFile("out.txt")

	if err != nil {
		return
	}

	lines := strings.Split(string(fileBytes), "\n")
	for i, line := range lines {
		plusIndex := strings.LastIndex(line, "+")

		if plusIndex != -1 {
			lines[i] = line[:plusIndex-1]
		}
	}

	os.WriteFile("out.txt", []byte(strings.Join(lines, "\n")), 0644)
}

//func main() {
//	cmd.Execute(version, artifactArch)
//}
