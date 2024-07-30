package utils

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	core "github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"golang.org/x/term"
)

var FlagsToIgnore = []string{"help", "version", "logLevel"}

func CreateDirectory(filename string) error {
	dir := filepath.Dir(filename)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err := os.MkdirAll(dir, 0o755)
		if err != nil {
			return err
		}
	}
	return nil
}

func IsInteractive() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	return nil
}

func MoveFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	err = os.Remove(src)
	return err
}

func CapitalizeFirst(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func FileExists(file string) bool {
	if absPath, err := filepath.Abs(file); err == nil {
		file = absPath
	}

	info, err := os.Stat(file)
	if os.IsNotExist(err) {
		return false
	}

	return !info.IsDir()
}

func SanitizeFilePath(path string) string {
	sanitizedPath := path
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path
		}

		sanitizedPath = filepath.Join(homeDir, path[2:])
	}

	if absPath, err := filepath.Abs(sanitizedPath); err == nil {
		sanitizedPath = absPath
	}

	return sanitizedPath
}

func GetWorkflow() (*workflow.Workflow, string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, "", err
	}

	wf, workflowFileLocation, err := workflow.Load(wd)
	if err != nil {
		return nil, "", err
	}

	return wf, workflowFileLocation, nil
}

func GetWorkflowAndDir() (*workflow.Workflow, string, error) {
	wf, wfFileLocation, err := GetWorkflow()
	if err != nil {
		return nil, "", err
	}

	// Get the project directory which is the parent of the .speakeasy folder the workflow file is in
	projectDir := filepath.Dir(filepath.Dir(wfFileLocation))
	if err := os.Chdir(projectDir); err != nil {
		return nil, "", err
	}

	return wf, projectDir, nil
}

func GetFullCommandString(cmd *cobra.Command) string {
	return strings.Join(GetCommandParts(cmd), " ")
}

func GetCommandParts(cmd *cobra.Command) []string {
	parts := strings.Split(cmd.CommandPath(), " ")
	for _, f := range getSetFlags(cmd.Flags()) {
		fval := f.Value.String()
		switch f.Value.Type() {
		case "stringSlice":
			fval = fval[1 : len(fval)-1] // Remove brackets
		}
		parts = append(parts, fmt.Sprintf("--%s=%s", f.Name, fval))
	}
	return parts
}

func getSetFlags(flags *pflag.FlagSet) []*pflag.Flag {
	values := make([]*pflag.Flag, 0)

	flags.VisitAll(func(flag *pflag.Flag) {
		if flag.Changed {
			values = append(values, flag)
		}
	})

	return values
}

// For these customers we limit callbacks to the speakeasy server outside of auth
func IsZeroTelemetryOrganization(ctx context.Context) bool {
	return core.IsTelemetryDisabled(ctx)
}

var yamlExtensions = []string{".yaml", ".yml"}

func HasYAMLExt(path string) bool {
	return slices.Contains(yamlExtensions, filepath.Ext(path))
}

func ReadFileToString(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(data), nil
}
