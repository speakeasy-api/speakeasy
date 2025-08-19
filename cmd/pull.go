package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"archive/zip"
	"bytes"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy-core/loader"
	"github.com/speakeasy-api/speakeasy-core/ocicommon"
	charm_internal "github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/sdk"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"oras.land/oras-go/v2/registry/remote"
	orasauth "oras.land/oras-go/v2/registry/remote/auth"
)

// Item represents a list item
type Item struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// FilterValue implements list.Item interface
func (i Item) FilterValue() string {
	return i.Name
}

// ItemDelegate handles rendering of list items
type ItemDelegate struct{}

func (d ItemDelegate) Height() int                               { return 1 }
func (d ItemDelegate) Spacing() int                              { return 0 }
func (d ItemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d ItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(Item)
	if !ok {
		return
	}

	str := fmt.Sprintf("%d. %s", index+1, i.Name)

	fn := lipgloss.NewStyle().PaddingLeft(4).Render
	if index == m.Index() {
		fn = lipgloss.NewStyle().
			PaddingLeft(4).
			Foreground(lipgloss.Color("170")).
			Render
	}

	fmt.Fprint(w, fn(str))
}

type pullModel struct {
	specsList     list.Model
	revisionsList list.Model
	outputDir     textinput.Model

	// state
	selectedSpec      Item
	selectedRevision  Item
	selectedOutputDir string

	// loading states
	loadingSpecs     bool
	loadingRevisions bool

	// Current step
	step int

	spinner spinner.Model
}

type pullFlags struct {
	Spec      string `json:"spec"`
	Revision  string `json:"revision"`
	OutputDir string `json:"output-dir"`
}

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "pull a spec from the registry",
	Long:  "pull a spec from the registry",
	RunE:  pullExec,
}

func pullInit() {
	pullCmd.Flags().String("spec", "", "The name of the spec to want to pull")
	pullCmd.Flags().String("revision", "latest", "The revision to pull")
	pullCmd.Flags().String("output-dir", getCurrentWorkingDirectory(), "The directory to output the image to")

	rootCmd.AddCommand(pullCmd)
}

func pullExec(cmd *cobra.Command, args []string) error {
	// find all flags that are not set, and run them interactively.
	// then pass them down to the actual runner.
	missingFlags := []string{}

	flags := cmd.Flags()
	flags.VisitAll(func(f *pflag.Flag) {
		if slices.Contains([]string{"help", "version", "logLevel"}, f.Name) {
			return
		}

		if !f.Changed {
			missingFlags = append(missingFlags, f.Name)
		}
	})

	// Initialize spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// initialise specs list
	specsList := list.New([]list.Item{}, ItemDelegate{}, 10, 20)
	specsList.Title = "All specs"
	specsList.SetShowTitle(false)
	// remove specsList subtitle

	// initialise revisions list
	revisionsList := list.New([]list.Item{}, ItemDelegate{}, 10, 20)
	revisionsList.Title = "All tags in spec namespace"
	revisionsList.SetShowTitle(false)

	// initialise output dir input
	outputDir := textinput.New()
	outputDir.Placeholder = "Enter the directory to output the image to"
	outputDir.Prompt = "Output directory: "
	outputDir.Focus()

	// run the model
	model := pullModel{
		specsList:        specsList,
		revisionsList:    revisionsList,
		outputDir:        outputDir,
		step:             1,
		loadingSpecs:     true,
		loadingRevisions: false,
		spinner:          s,
	}

	filledValues := model.Run()

	fmt.Println(filledValues)

	runPull(cmd.Context(), pullFlags{
		Spec:      filledValues["spec"],
		Revision:  filledValues["revision"],
		OutputDir: filledValues["output-dir"],
	})

	return nil
}

func (m *pullModel) Init() tea.Cmd {
	return tea.Batch(
		getNamepacesCmd(), m.spinner.Tick,
	)
}

func (m *pullModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			switch m.step {
			case 1:
				if !m.loadingSpecs && m.specsList.SelectedItem() != nil {
					m.selectedSpec = m.specsList.SelectedItem().(Item)
					m.step = 2
					m.loadingRevisions = true
					return m, getTagsCmd(m.selectedSpec.Name)
				}
			case 2:
				if !m.loadingRevisions && m.revisionsList.SelectedItem() != nil {
					m.selectedRevision = m.revisionsList.SelectedItem().(Item)
					m.step = 3
					return m, m.outputDir.Cursor.BlinkCmd()
				}
			case 3:
				m.selectedOutputDir = m.outputDir.Value()
				m.step = 4
				return m, nil
			case 4:
				return m, tea.Quit
			}
		}
	case getNamespacesMsg:
		m.loadingSpecs = false
		var items []list.Item
		for _, item := range msg.items {
			items = append(items, item)
		}
		m.specsList.SetItems(items)
	case getTagsMsg:
		m.loadingRevisions = false
		var items []list.Item
		for _, item := range msg.items {
			items = append(items, item)
		}
		m.revisionsList.SetItems(items)
	}

	switch m.step {
	case 1:
		if !m.loadingSpecs {
			var cmd tea.Cmd
			m.specsList, cmd = m.specsList.Update(msg)
			cmds = append(cmds, cmd)
		}
	case 2:
		if !m.loadingRevisions {
			var cmd tea.Cmd
			m.revisionsList, cmd = m.revisionsList.Update(msg)
			cmds = append(cmds, cmd)
		}
	case 3:
		var cmd tea.Cmd
		m.outputDir, cmd = m.outputDir.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Update spinner
	var spinnerCmd tea.Cmd
	m.spinner, spinnerCmd = m.spinner.Update(msg)
	cmds = append(cmds, spinnerCmd)

	return m, tea.Batch(cmds...)
}

func (m *pullModel) HandleKeypress(key string) tea.Cmd {
	return nil
}

func (m *pullModel) View() string {
	var s strings.Builder

	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1).
		Render("speakeasy pull")
	s.WriteString(title + "\n\n")

	switch m.step {
	case 1:
		if m.loadingSpecs {
			s.WriteString(fmt.Sprintf("Loading specs... %s", m.spinner.View()))
		} else {
			s.WriteString(m.specsList.View())
		}
	case 2:
		if m.loadingRevisions {
			s.WriteString(fmt.Sprintf("Loading revisions... %s", m.spinner.View()))
		} else {
			s.WriteString(fmt.Sprintf("Selected spec: %s\n", m.selectedSpec.Name))
			s.WriteString(m.revisionsList.View())
		}
	case 3:
		s.WriteString(fmt.Sprintf("Selected spec: %s\n", m.selectedSpec.Name))
		s.WriteString(fmt.Sprintf("Selected revision: %s\n", m.selectedRevision.Name))
		s.WriteString(m.outputDir.View())
	case 4:
		s.WriteString(fmt.Sprintf("Selected spec: %s\n", m.selectedSpec.Name))
		s.WriteString(fmt.Sprintf("Selected revision: %s\n", m.selectedRevision.Name))
		s.WriteString(fmt.Sprintf("Selected output directory: %s\n", m.selectedOutputDir))
		// add a clickable button to run the pull
		button := interactivity.Button{
			Label:    "Pull",
			Disabled: false,
			Hovered:  false,
			Clicked:  false,
		}
		s.WriteString(button.View())
	}

	return s.String()
}

func (m *pullModel) OnUserExit() {}

func (m *pullModel) SetWidth(width int) {
}

func (m *pullModel) getFilledValues() map[string]string {
	inputResults := make(map[string]string)

	inputResults["spec"] = m.selectedSpec.Name
	inputResults["revision"] = m.selectedRevision.Name
	inputResults["output-dir"] = m.selectedOutputDir

	return inputResults
}

func (m *pullModel) Run() map[string]string {
	newM, err := charm_internal.RunModel(m)
	if err != nil {
		os.Exit(1)
	}

	resultingModel := newM.(*pullModel)

	return resultingModel.getFilledValues()
}

func runPull(ctx context.Context, flags pullFlags) error {
	logger := log.From(ctx)

	logger.Infof("Pulling from spec: %s", flags.Spec)

	// Get server URL and determine if insecure
	serverURL := auth.GetServerURL()
	insecurePublish := false
	if strings.HasPrefix(serverURL, "http://") {
		insecurePublish = true
	}
	reg := strings.TrimPrefix(serverURL, "http://")
	reg = strings.TrimPrefix(reg, "https://")

	// Get API key
	apiKey := config.GetSpeakeasyAPIKey()
	if apiKey == "" {
		return fmt.Errorf("no API key available, please run 'speakeasy auth' to authenticate")
	}

	// Create repository access
	access := ocicommon.NewRepositoryAccess(apiKey, flags.Spec, ocicommon.RepositoryAccessOptions{
		Insecure: insecurePublish,
	})

	// Create bundle loader
	bundleLoader := loader.NewLoader(loader.OCILoaderOptions{
		Registry: reg,
		Access:   access,
	})

	// Load the OpenAPI bundle
	bundleResult, err := bundleLoader.LoadOpenAPIBundle(ctx, flags.Revision)
	if err != nil {
		logger.Errorf("Error loading OCI image by revision %s: %v", flags.Revision, err)
		return fmt.Errorf("failed to load bundle: %w", err)
	}

	defer bundleResult.Body.Close()

	// Create output directory
	if err := os.MkdirAll(flags.OutputDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Extract bundle to output directory
	if err := extractBundle(bundleResult, flags.OutputDir); err != nil {
		return fmt.Errorf("failed to extract bundle: %w", err)
	}

	logger.Infof("Successfully pulled bundle to %s", flags.OutputDir)
	return nil
}

func extractBundle(bundleResult *loader.OpenAPIBundleResult, outputDir string) error {
	buf, err := io.ReadAll(bundleResult.Body)
	if err != nil {
		return fmt.Errorf("failed to read bundle content: %w", err)
	}

	// Create zip reader
	reader := bytes.NewReader(buf)
	zipReader, err := zip.NewReader(reader, int64(len(buf)))
	if err != nil {
		return fmt.Errorf("failed to create zip reader: %w", err)
	}

	// Extract files
	for _, file := range zipReader.File {
		cleanName := filepath.Clean(file.Name)
		filePath := filepath.Join(outputDir, cleanName)

		// Security check to prevent path traversal
		if !strings.HasPrefix(filePath, filepath.Clean(outputDir)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", filePath)
		}

		// Create directory if needed
		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		// Skip if it's a directory
		if file.FileInfo().IsDir() {
			continue
		}

		// Create file
		dst, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		defer dst.Close()

		// Open source file
		src, err := file.Open()
		if err != nil {
			return fmt.Errorf("failed to open source file: %w", err)
		}
		defer src.Close()

		// Copy content
		if _, err := io.Copy(dst, src); err != nil {
			return fmt.Errorf("failed to copy file content: %w", err)
		}
	}

	return nil
}

func getCurrentWorkingDirectory() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	return cwd
}

type getNamespacesMsg struct{ items []Item }

func getNamepacesCmd() tea.Cmd {
	return func() tea.Msg {
		namespaces, err := getNamespaces()
		if err != nil {
			return err
		}

		items := []Item{}
		for _, namespace := range namespaces {
			items = append(items, Item{ID: namespace, Name: namespace})
		}

		return getNamespacesMsg{items: items}
	}
}

func getNamespaces() ([]string, error) {
	// Initialize speakeasy client
	client, err := sdk.InitSDK()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize speakeasy client: %w", err)
	}

	// Get targets from the events API
	res, err := client.Events.GetTargets(context.Background(), operations.GetWorkspaceTargetsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to get targets: %w", err)
	}

	// Extract unique namespaces from targets
	seenNamespaces := make(map[string]bool)
	var namespaces []string

	for _, target := range res.TargetSDKList {
		if target.SourceNamespaceName != nil && !seenNamespaces[*target.SourceNamespaceName] {
			seenNamespaces[*target.SourceNamespaceName] = true
			namespaces = append(namespaces, *target.SourceNamespaceName)
		}
	}

	return namespaces, nil
}

type getTagsMsg struct{ items []Item }

func getTagsCmd(namespace string) tea.Cmd {
	return func() tea.Msg {
		tags, err := getTags(namespace)
		if err != nil {
			return err
		}

		items := []Item{}
		for _, tag := range tags {
			items = append(items, Item{ID: tag, Name: tag})
		}

		return getTagsMsg{items: items}
	}
}

// getTags connects to a remote OCI registry and retrieves all tags for a given repository.
// It takes a context and the repository name (e.g., "ghcr.io/oras-project/oras-go-demo") as input.
// It returns a slice of strings containing all the tags, or an error if one occurred.
func getTags(namespace string) ([]string, error) {
	// Get server URL and determine if insecure
	serverURL := auth.GetServerURL()
	insecurePublish := false
	if strings.HasPrefix(serverURL, "http://") {
		insecurePublish = true
	}
	reg := strings.TrimPrefix(serverURL, "http://")
	reg = strings.TrimPrefix(reg, "https://")

	// Get API key
	apiKey := config.GetSpeakeasyAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("no API key available, please run 'speakeasy auth' to authenticate")
	}

	// Create repository access
	access := ocicommon.NewRepositoryAccess(apiKey, namespace, ocicommon.RepositoryAccessOptions{
		Insecure: insecurePublish,
	})
	accessResult, err := access.Acquire(context.Background(), reg)
	if err != nil {
		return nil, fmt.Errorf("error acquiring oci access: %w", err)
	}

	repositoryURL := path.Join(reg, accessResult.Repository)

	// Create a new instance of a remote repository client.
	repo, err := remote.NewRepository(repositoryURL)
	if err != nil {
		return nil, fmt.Errorf("error creating remote repository client: %w", err)
	}

	rh := retryablehttp.NewClient()

	// TODO: remove this once we have a logger
	rh.Logger = nil
	repo.Client = access.WrapClient(&orasauth.Client{
		Client:     rh.StandardClient(),
		Header:     orasauth.DefaultClient.Header,
		Cache:      orasauth.NewCache(),
		Credential: accessResult.CredentialFunc,
	})

	var allTags []string
	// The `Tags` method paginates through the tags from the registry.
	// We provide a callback function that appends each page of tags to our slice.
	err = repo.Tags(context.Background(), "", func(tags []string) error {
		allTags = append(allTags, tags...)
		// Return nil to continue fetching subsequent pages.
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list tags for repository %s: %w", repositoryURL, err)
	}

	return allTags, nil
}
