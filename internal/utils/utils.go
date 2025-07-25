package utils

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"unicode"

	"github.com/charmbracelet/glamour"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	core "github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"golang.org/x/term"
)

var renderer *glamour.TermRenderer

func init() {
	var err error
	renderer, err = glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(120),
	)
	if err != nil {
		panic(err)
	}
}

func RenderMarkdown(md string) string {

	if env.IsDocsRuntime() {
		return md
	}

	text, err := renderer.Render(md)
	if err != nil {

		panic(err)
	}
	return text
}

func RenderError(md string) string {
	return RenderMarkdown("_Error_:\n" + md)
}

func OpenInBrowser(path string) error {
	var err error

	url := path

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}

	return err
}

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
	if os.IsNotExist(err) || info == nil {
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

func WriteStringToFile(path, content string) error {
	if err := CreateDirectory(path); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(content), 0o644)
}

// Returns full path to the temp file
func WriteTempFile(content string, optionalFilename string) (string, error) {
	fileName := optionalFilename
	if fileName == "" {
		fileName = "tempfile"
	}

	tmpFile, err := os.CreateTemp("", fileName)
	if err != nil {
		return "", err
	}

	if _, err := tmpFile.WriteString(content); err != nil {
		return "", err
	}

	if err := tmpFile.Close(); err != nil {
		return "", err
	}

	return tmpFile.Name(), nil
}

// Single-Slot-Queue (aka "latest arrived cheats")
// - only one function can run at a the same time
// - there can only be one function queued at a time
// - when a new function is queued it replaces the previous one in the queue
// - if a bump function is provided, it is run immediately upon enqueuing
func SingleSlotQueue(bump func()) func(fn func()) {
	mu := sync.Mutex{}

	var queued func()
	var running bool

	return func(fn func()) {
		mu.Lock()
		if running {
			queued = fn
			mu.Unlock()
			if bump != nil {
				bump()
			}
			return
		}

		running = true
		mu.Unlock()

		for {
			fn()

			mu.Lock()
			if queued == nil {
				running = false
				mu.Unlock()
				return
			}

			fn = queued
			queued = nil
			mu.Unlock()
		}
	}
}
