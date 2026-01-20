package cmd

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	speakeasyclientsdkgo "github.com/speakeasy-api/speakeasy-client-sdk-go/v3"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	core "github.com/speakeasy-api/speakeasy-core/auth"
	"golang.org/x/sync/errgroup"

	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/internal/links"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/sdk"
	"github.com/speakeasy-api/speakeasy/internal/utils"
)

type statusFlagsArgs struct {
	Output string `json:"output"`
}

var statusCmd = &model.ExecutableCommand[statusFlagsArgs]{
	Usage:        "status",
	Short:        "Review status of current workspace",
	Run:          runStatus,
	RequiresAuth: true,
	Flags: []flag.Flag{
		flag.EnumFlag{
			Name:          "output",
			Shorthand:     "o",
			Description:   "Output format (summary: visual boxes, console: plain text for non-TTY environments, json: structured JSON for automation)",
			AllowedValues: []string{"summary", "console", "json"},
			DefaultValue:  "summary",
		},
	},
}

func runStatus(ctx context.Context, flags statusFlagsArgs) error {
	client, err := sdk.InitSDK()
	if err != nil {
		return fmt.Errorf("error initializing Speakeasy client: %w", err)
	}

	model, err := newStatusModel(ctx, client, flags.Output)
	if err != nil {
		return err
	}

	switch flags.Output {
	case "json":
		return model.PrintJSON(ctx)
	case "console":
		model.PrintConsole(ctx)
	default:
		model.Print(ctx)
	}

	return nil
}

var skipMeaninglessTargetNames = []string{
	"",
	"first-target",
	"my-first-target",
}

type statusModel struct {
	organization statusOrganizationModel
	workspace    statusWorkspaceModel
}

func newStatusModel(ctx context.Context, client *speakeasyclientsdkgo.Speakeasy, outputMode string) (statusModel, error) {
	var result statusModel

	workspaceID, err := core.GetWorkspaceIDFromContext(ctx)
	if err != nil {
		return result, err
	}

	configWorkspaceID := config.GetWorkspaceID()

	if !slices.Contains([]string{"", "self"}, configWorkspaceID) {
		workspaceID = configWorkspaceID
	}

	wsReq := operations.GetWorkspaceRequest{
		WorkspaceID: &workspaceID,
	}

	wsRes, err := client.Workspaces.GetByID(ctx, wsReq)
	if err != nil {
		return result, fmt.Errorf("error getting Speakeasy workspace: %w", err)
	}

	if wsRes.StatusCode != http.StatusOK {
		return result, fmt.Errorf("unexpected status code getting Speakeasy workspace: %d", wsRes.StatusCode)
	}

	if wsRes.Workspace == nil {
		return result, fmt.Errorf("unexpected missing workspace response")
	}

	// Only log in non-JSON modes
	if outputMode != "json" {
		log.From(ctx).Printf("Workspace: %s", wsRes.Workspace.Name)
	}

	orgReq := operations.GetOrganizationRequest{
		OrganizationID: wsRes.Workspace.OrganizationID,
	}

	orgRes, err := client.Organizations.Get(ctx, orgReq)
	if err != nil {
		return result, fmt.Errorf("error getting Speakeasy organization: %w", err)
	}

	if orgRes.StatusCode != http.StatusOK {
		return result, fmt.Errorf("unexpected status code getting Speakeasy organization: %d", orgRes.StatusCode)
	}

	if orgRes.Organization == nil {
		return result, fmt.Errorf("unexpected missing organization response")
	}

	result.organization = newStatusOrganizationModel(ctx, client, *orgRes.Organization)

	workspace, err := newStatusWorkspaceModel(ctx, client, result.organization, *wsRes.Workspace, outputMode)
	if err != nil {
		return result, err
	}

	result.workspace = workspace

	return result, nil
}

func (m statusModel) Print(ctx context.Context) {
	logger := log.From(ctx)

	overviewLines := []string{
		fmt.Sprintf("Workspace: %s/%s", m.organization.Name(), m.workspace.Name()),
		m.organization.AccountTypeLine(),
	}

	logger.Println(renderOverviewBox(overviewLines...))

	m.workspace.targets.Print(ctx)
}

func (m statusModel) PrintConsole(ctx context.Context) {
	logger := log.From(ctx)

	logger.Println("// SPEAKEASY")
	logger.Printf("Workspace: %s/%s", m.organization.Name(), m.workspace.Name())
	logger.Println(m.organization.AccountTypeLine())
	logger.Println("")

	m.workspace.targets.PrintConsole(ctx)
}

// JSON output types
type statusJSONOutput struct {
	Workspace statusJSONWorkspace `json:"workspace"`
	Targets   statusJSONTargets   `json:"targets"`
}

type statusJSONWorkspace struct {
	Name        string  `json:"name"`
	AccountType string  `json:"accountType"`
	TrialExpiry *string `json:"trialExpiry,omitempty"`
}

type statusJSONTargets struct {
	Published    []statusJSONTarget `json:"published"`
	Configured   []statusJSONTarget `json:"configured"`
	Unconfigured []statusJSONTarget `json:"unconfigured"`
}

type statusJSONTarget struct {
	Name           string                  `json:"name"`
	Language       string                  `json:"language"`
	SDKVersion     string                  `json:"sdkVersion"`
	Status         string                  `json:"status"`
	Version        string                  `json:"version,omitempty"`
	PublishURL     string                  `json:"publishURL,omitempty"`
	RepositoryURL  string                  `json:"repositoryURL,omitempty"`
	SpeakeasyURL   string                  `json:"speakeasyURL"`
	LastPublish    *statusJSONTimestamp    `json:"lastPublish,omitempty"`
	LastGenerate   *statusJSONTimestamp    `json:"lastGenerate,omitempty"`
	UpgradeURL     string                  `json:"upgradeURL,omitempty"`
	GenerateFailed *statusJSONGenerateFail `json:"generateFailed,omitempty"`
}

type statusJSONTimestamp struct {
	Timestamp string `json:"timestamp"`
	Actor     string `json:"actor"`
	Version   string `json:"version,omitempty"`
	Location  string `json:"location,omitempty"`
}

type statusJSONGenerateFail struct {
	Failed  bool   `json:"failed"`
	RunLink string `json:"runLink,omitempty"`
}

func (m statusModel) PrintJSON(ctx context.Context) error {
	output := statusJSONOutput{
		Workspace: statusJSONWorkspace{
			Name:        fmt.Sprintf("%s/%s", m.organization.Name(), m.workspace.Name()),
			AccountType: m.organization.accountType,
		},
		Targets: statusJSONTargets{
			Published:    m.workspace.targets.toJSONTargets(ctx, m.workspace.targets.published),
			Configured:   m.workspace.targets.toJSONTargets(ctx, m.workspace.targets.configured),
			Unconfigured: m.workspace.targets.toJSONTargets(ctx, m.workspace.targets.unconfigured),
		},
	}

	if m.organization.freeTrialExpiry != nil {
		expiry := m.organization.freeTrialExpiry.Format(time.RFC3339)
		output.Workspace.TrialExpiry = &expiry
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func (m statusWorkspaceTargetsModel) toJSONTargets(ctx context.Context, targets []statusWorkspaceTargetModel) []statusJSONTarget {
	result := make([]statusJSONTarget, 0, len(targets))
	for _, t := range targets {
		result = append(result, t.toJSON(ctx))
	}
	return result
}

func (m statusWorkspaceTargetModel) toJSON(ctx context.Context) statusJSONTarget {
	// Determine status
	var status string
	switch {
	case m.generatePublished != nil && *m.generatePublished:
		status = "Published"
	case m.ghActionOrganization != nil && m.ghActionRepository != nil:
		status = "Unpublished"
	default:
		status = "Unconfigured"
	}

	// Determine version
	var version string
	if event := m.workspaceEventCompilation; event != nil && event.publishPackageName != nil && event.publishPackageVersion != nil {
		version = *event.publishPackageVersion
	} else if m.generateConfigPostVersion != nil {
		version = *m.generateConfigPostVersion
	} else if m.generateGenLockPreVersion != nil {
		version = *m.generateGenLockPreVersion
	}

	// Determine SDK version
	sdkVersion := "v1"
	if m.generateTargetVersion != nil && *m.generateTargetVersion != "" {
		sdkVersion = *m.generateTargetVersion
	}

	// Determine name
	name := m.generateTarget
	if m.generateTargetName != nil && !slices.Contains(skipMeaninglessTargetNames, *m.generateTargetName) {
		name = *m.generateTargetName
	}

	result := statusJSONTarget{
		Name:         name,
		Language:     m.generateTarget,
		SDKVersion:   sdkVersion,
		Status:       status,
		Version:      version,
		SpeakeasyURL: m.speakeasyURL,
	}

	if m.RepositoryURL() != "" {
		result.RepositoryURL = m.RepositoryURL()
	}

	if m.workspaceEventCompilation != nil && m.workspaceEventCompilation.publishPackageURL != nil {
		result.PublishURL = *m.workspaceEventCompilation.publishPackageURL
	}

	if m.upgradeDocumentationURL != nil {
		result.UpgradeURL = *m.upgradeDocumentationURL
	}

	// Generate failed info
	if m.workspaceEventCompilation != nil && !m.workspaceEventCompilation.success {
		fail := &statusJSONGenerateFail{Failed: true}
		if m.workspaceEventCompilation.ghActionRunLink != nil {
			fail.RunLink = links.Shorten(ctx, *m.workspaceEventCompilation.ghActionRunLink)
		}
		result.GenerateFailed = fail
	}

	// Last publish
	if m.workspaceEventCompilation != nil && m.workspaceEventCompilation.publishPackageName != nil {
		result.LastPublish = &statusJSONTimestamp{
			Timestamp: m.workspaceEventCompilation.lastPublishCreatedAt.Format(time.RFC3339),
			Actor:     m.getActor(),
		}
		if m.workspaceEventCompilation.publishPackageVersion != nil {
			result.LastPublish.Version = *m.workspaceEventCompilation.publishPackageVersion
		}
	}

	// Last generate
	if m.workspaceEventCompilation != nil {
		gen := &statusJSONTimestamp{
			Timestamp: m.workspaceEventCompilation.createdAt.Format(time.RFC3339),
			Actor:     m.getActor(),
		}
		if m.ghActionRunLink == nil {
			gen.Location = "local"
		} else {
			gen.Location = "ci"
		}
		result.LastGenerate = gen
	}

	return result
}

func (m statusWorkspaceTargetModel) getActor() string {
	switch {
	case m.ghActionRunLink != nil:
		return "GitHub Actions"
	case m.continuousIntegrationEnvironment != nil:
		return "CI"
	case m.gitUserName != nil:
		return *m.gitUserName
	case m.gitUserEmail != nil:
		return *m.gitUserEmail
	case m.hostname != nil:
		return *m.hostname
	default:
		return "Unknown"
	}
}

type statusOrganizationModel struct {
	accountType     string
	freeTrialExpiry *time.Time
	name            string
	slug            string
}

func newStatusOrganizationModel(_ context.Context, _ *speakeasyclientsdkgo.Speakeasy, organization shared.Organization) statusOrganizationModel {
	result := statusOrganizationModel{
		accountType:     string(organization.AccountType),
		freeTrialExpiry: organization.FreeTrialExpiry,
		name:            organization.Name,
		slug:            organization.Slug,
	}

	return result
}

func (m statusOrganizationModel) Name() string {
	if m.name != "" {
		return m.name
	}

	return m.slug
}

func (m statusOrganizationModel) AccountTypeLine() string {
	var accountTypeLine strings.Builder

	accountTypeLine.WriteString("Account Type: ")
	accountTypeLine.WriteString(m.accountType)

	if m.accountType == string(shared.AccountTypeFree) && m.freeTrialExpiry != nil {
		expiryDiff := time.Until(*m.freeTrialExpiry)
		expiryHours := int64(expiryDiff.Hours()) % 24
		expiryDays := int64(expiryDiff.Hours() / 24)

		accountTypeLine.WriteString(" (Business Trial Expire")

		if expiryHours > 0 {
			accountTypeLine.WriteString("s: ")
			accountTypeLine.WriteString(strconv.Itoa(int(expiryDays)))
			accountTypeLine.WriteString(" days ")
			accountTypeLine.WriteString(strconv.Itoa(int(expiryHours)))
			accountTypeLine.WriteString(" hours")
		} else {
			accountTypeLine.WriteString("d")
		}
		accountTypeLine.WriteString(")")
	}

	return accountTypeLine.String()
}

type statusWorkspaceModel struct {
	name           string
	id             string
	slug           string
	organizationID string
	targets        statusWorkspaceTargetsModel
}

func newStatusWorkspaceModel(ctx context.Context, client *speakeasyclientsdkgo.Speakeasy, org statusOrganizationModel, workspace shared.Workspace, outputMode string) (statusWorkspaceModel, error) {
	result := statusWorkspaceModel{
		id:             workspace.ID,
		name:           workspace.Name,
		organizationID: workspace.OrganizationID,
		slug:           workspace.Slug,
	}

	var stopSpinner func()
	switch outputMode {
	case "json":
		// Silent for JSON output
		stopSpinner = func() {}
	case "console":
		log.From(ctx).Println("Querying for active SDKs and other targets...")
		stopSpinner = func() {}
	default:
		stopSpinner = interactivity.StartSpinner("Querying for active SDKs and other targets...")
	}

	wsTargetsreq := operations.GetWorkspaceTargetsRequest{}

	wsTargetsRes, err := client.Events.GetTargets(ctx, wsTargetsreq)
	if err != nil {
		stopSpinner()
		return result, fmt.Errorf("error getting Speakeasy workspace targets: %w", err)
	}

	if wsTargetsRes.StatusCode != http.StatusOK {
		stopSpinner()
		return result, fmt.Errorf("unexpected status code getting Speakeasy workspace targets: %d", wsTargetsRes.StatusCode)
	}

	targets, err := newStatusWorkspaceTargetsModel(ctx, org, result, wsTargetsRes.TargetSDKList)

	stopSpinner()

	if err != nil {
		return result, err
	}

	result.targets = targets

	return result, nil
}

func (m statusWorkspaceModel) Name() string {
	if m.name != "" {
		return m.name
	}

	return m.slug
}

type statusWorkspaceTargetsModel struct {
	published    []statusWorkspaceTargetModel
	configured   []statusWorkspaceTargetModel
	unconfigured []statusWorkspaceTargetModel
}

func newStatusWorkspaceTargetsModel(ctx context.Context, org statusOrganizationModel, workspace statusWorkspaceModel, targets []shared.TargetSDK) (statusWorkspaceTargetsModel, error) {
	var result statusWorkspaceTargetsModel

	targetModels := make([]*statusWorkspaceTargetModel, len(targets))
	eg, ctx := errgroup.WithContext(ctx)

	for index, target := range targets {
		// Archived
		if target.LastEventInteractionType == shared.InteractionTypeTombstone {
			continue
		}

		eg.Go(func() error {
			targetModels[index] = newStatusWorkspaceTargetModel(ctx, org, workspace, target)

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return result, err
	}

	for _, targetModel := range targetModels {
		if targetModel == nil {
			continue
		}

		if targetModel.generatePublished != nil && *targetModel.generatePublished {
			result.published = append(result.published, *targetModel)

			continue
		}

		if targetModel.ghActionOrganization != nil && targetModel.ghActionRepository != nil {
			result.configured = append(result.configured, *targetModel)

			continue
		}

		result.unconfigured = append(result.unconfigured, *targetModel)
	}

	sortFunc := func(a, b statusWorkspaceTargetModel) int {
		return cmp.Or(
			cmp.Compare(strconv.FormatBool(a.Success()), strconv.FormatBool(b.Success())),
			cmp.Compare(a.TargetName(), b.TargetName()),
		)
	}

	slices.SortFunc(result.configured, sortFunc)
	slices.SortFunc(result.published, sortFunc)
	slices.SortFunc(result.unconfigured, sortFunc)

	return result, nil
}

func (m statusWorkspaceTargetsModel) Print(ctx context.Context) {
	logger := log.From(ctx)

	// Leave room for padding (if the terminal is too small to fit, we need to wrap)
	width := min(m.RenderFullWidth(ctx), styles.TerminalWidth()-2)

	for _, target := range m.published {
		if !target.Success() {
			logger.Println(renderPublishedErrorTargetBox(
				width,
				target.TargetHeading(),
				target.TargetInfo(ctx)...,
			))
		} else {
			logger.Println(renderPublishedSuccessTargetBox(
				width,
				target.TargetHeading(),
				target.TargetInfo(ctx)...,
			))
		}
	}

	for _, target := range m.configured {
		logger.Println(renderConfiguredTargetBox(
			width,
			target.TargetHeading(),
			target.TargetInfo(ctx)...,
		))
	}

	for _, target := range m.unconfigured {
		logger.Println(renderUnconfiguredTargetBox(
			width,
			target.TargetHeading(),
			target.TargetInfo(ctx)...,
		))
	}
}

func (m statusWorkspaceTargetsModel) PrintConsole(ctx context.Context) {
	logger := log.From(ctx)

	if len(m.published) > 0 {
		logger.Printf("Published Targets (%d)", len(m.published))
		logger.Println(strings.Repeat("=", 25))
		for i, target := range m.published {
			target.PrintConsole(ctx)
			if i < len(m.published)-1 {
				logger.Println("")
			}
		}
		logger.Println("")
	}

	if len(m.configured) > 0 {
		logger.Printf("Configured Targets (%d)", len(m.configured))
		logger.Println(strings.Repeat("=", 25))
		for i, target := range m.configured {
			target.PrintConsole(ctx)
			if i < len(m.configured)-1 {
				logger.Println("")
			}
		}
		logger.Println("")
	}

	if len(m.unconfigured) > 0 {
		logger.Printf("Unconfigured Targets (%d)", len(m.unconfigured))
		logger.Println(strings.Repeat("=", 25))
		for i, target := range m.unconfigured {
			target.PrintConsole(ctx)
			if i < len(m.unconfigured)-1 {
				logger.Println("")
			}
		}
		logger.Println("")
	}

	if len(m.published) == 0 && len(m.configured) == 0 && len(m.unconfigured) == 0 {
		logger.Println("No targets found.")
	}
}

func (m statusWorkspaceTargetsModel) RenderFullWidth(ctx context.Context) int {
	var width int

	for _, target := range m.published {
		width = max(width, target.RenderFullWidth(ctx))
	}

	for _, target := range m.configured {
		width = max(width, target.RenderFullWidth(ctx))
	}

	for _, target := range m.unconfigured {
		width = max(width, target.RenderFullWidth(ctx))
	}

	return width
}

type statusWorkspaceEventModel struct {
	// Passthrough from search events API

	continuousIntegrationEnvironment *string
	createdAt                        time.Time
	ghActionRunLink                  *string
	gitUserEmail                     *string
	gitUserName                      *string
	hostname                         *string
	publishPackageName               *string
	publishPackageRegistryName       *string
	publishPackageURL                *string
	publishPackageVersion            *string
	lastPublishCreatedAt             time.Time
	success                          bool
}

func newStatusWorkspaceEventModel(target shared.TargetSDK) *statusWorkspaceEventModel {
	result := &statusWorkspaceEventModel{
		continuousIntegrationEnvironment: target.ContinuousIntegrationEnvironment,
		createdAt:                        target.LastEventCreatedAt,
		ghActionRunLink:                  target.GhActionRunLink,
		gitUserEmail:                     target.GitUserEmail,
		gitUserName:                      target.GitUserName,
		hostname:                         target.Hostname,
		publishPackageName:               target.PublishPackageName,
		publishPackageRegistryName:       target.PublishPackageRegistryName,
		publishPackageURL:                target.PublishPackageURL,
		publishPackageVersion:            target.PublishPackageVersion,
		success:                          target.Success != nil && *target.Success,
	}

	if target.LastPublishCreatedAt != nil {
		result.lastPublishCreatedAt = *target.LastPublishCreatedAt
	}

	return result
}

func (m statusWorkspaceEventModel) GenerateInfo() string {
	var result strings.Builder

	if m.ghActionRunLink == nil {
		result.WriteString("locally at ")
	}

	result.WriteString(m.createdAt.Format(time.RFC3339))
	result.WriteString(" by ")

	switch {
	case m.ghActionRunLink != nil:
		result.WriteString("GitHub Actions")
	case m.continuousIntegrationEnvironment != nil:
		result.WriteString("CI")
	case m.gitUserName != nil:
		result.WriteString(*m.gitUserName)
	case m.gitUserEmail != nil:
		result.WriteString(*m.gitUserEmail)
	case m.hostname != nil:
		result.WriteString(*m.hostname)
	default:
		result.WriteString("Unknown")
	}

	return result.String()
}

func (m statusWorkspaceEventModel) PublishInfo() string {
	var result strings.Builder

	if m.publishPackageVersion != nil {
		result.WriteString(*m.publishPackageVersion)
		result.WriteString(" at ")
	}

	if m.ghActionRunLink == nil {
		result.WriteString("locally at ")
	}

	result.WriteString(m.lastPublishCreatedAt.Format(time.RFC3339))
	result.WriteString(" by ")

	switch {
	case m.ghActionRunLink != nil:
		result.WriteString("GitHub Actions")
	case m.continuousIntegrationEnvironment != nil:
		result.WriteString("CI")
	case m.gitUserName != nil:
		result.WriteString(*m.gitUserName)
	case m.gitUserEmail != nil:
		result.WriteString(*m.gitUserEmail)
	case m.hostname != nil:
		result.WriteString(*m.hostname)
	default:
		result.WriteString("Unknown")
	}

	return result.String()
}

type statusWorkspaceTargetModel struct {
	// Passthrough from targets API

	continuousIntegrationEnvironment  *string
	generateConfigPostVersion         *string
	generateGenLockPreVersion         *string
	generateNumberOfOperationsIgnored *int64
	generateNumberOfOperationsUsed    *int64
	generatePublished                 *bool
	generateTarget                    string
	generateTargetName                *string
	generateTargetVersion             *string
	ghActionOrganization              *string
	ghActionRepository                *string
	ghActionRunLink                   *string
	gitUserEmail                      *string
	gitUserName                       *string
	hostname                          *string
	id                                string
	lastEventCreatedAt                time.Time
	success                           *bool

	workspaceEventCompilation *statusWorkspaceEventModel

	// Speakeasy URL
	speakeasyURL string

	// If a target upgrade is recommended, the URL for upgrade documentation.
	upgradeDocumentationURL *string
}

func newStatusWorkspaceTargetModel(ctx context.Context, org statusOrganizationModel, workspace statusWorkspaceModel, target shared.TargetSDK) *statusWorkspaceTargetModel {
	result := &statusWorkspaceTargetModel{
		continuousIntegrationEnvironment:  target.ContinuousIntegrationEnvironment,
		generateConfigPostVersion:         target.GenerateConfigPostVersion,
		generateGenLockPreVersion:         target.GenerateGenLockPreVersion,
		generateNumberOfOperationsIgnored: target.GenerateNumberOfOperationsIgnored,
		generateNumberOfOperationsUsed:    target.GenerateNumberOfOperationsUsed,
		generatePublished:                 target.GeneratePublished,
		generateTarget:                    target.GenerateTarget,
		generateTargetName:                target.GenerateTargetName,
		generateTargetVersion:             target.GenerateTargetVersion,
		ghActionOrganization:              target.GhActionOrganization,
		ghActionRepository:                target.GhActionRepository,
		ghActionRunLink:                   target.GhActionRunLink,
		gitUserEmail:                      target.GitUserEmail,
		gitUserName:                       target.GitUserName,
		hostname:                          target.Hostname,
		id:                                target.ID,
		lastEventCreatedAt:                target.LastEventCreatedAt,
		speakeasyURL:                      links.Shorten(ctx, fmt.Sprintf("https://app.speakeasy.com/org/%s/%s/targets/%s", org.slug, workspace.slug, target.ID)),
		success:                           target.Success,
	}

	if result.generateTarget == "python" && (result.generateTargetVersion == nil || *result.generateTargetVersion == "v1") {
		upgradeURL := "https://www.speakeasy.com/docs/python-migration"
		result.upgradeDocumentationURL = &upgradeURL
	}

	result.workspaceEventCompilation = newStatusWorkspaceEventModel(target)
	return result
}

func (m statusWorkspaceTargetModel) GenerateInfo() string {
	var result strings.Builder

	if m.ghActionRunLink == nil {
		result.WriteString("locally at ")
	}

	result.WriteString(m.lastEventCreatedAt.Format(time.RFC3339))
	result.WriteString(" by ")

	switch {
	case m.ghActionRunLink != nil:
		result.WriteString("GitHub Actions")
	case m.continuousIntegrationEnvironment != nil:
		result.WriteString("CI")
	case m.gitUserName != nil:
		result.WriteString(*m.gitUserName)
	case m.gitUserEmail != nil:
		result.WriteString(*m.gitUserEmail)
	case m.hostname != nil:
		result.WriteString(*m.hostname)
	default:
		result.WriteString("Unknown")
	}

	return result.String()
}

func (m statusWorkspaceTargetModel) RenderFullWidth(ctx context.Context) int {
	// Add 2 to account for box padding
	return lipgloss.Width(strings.Join(m.TargetInfo(ctx), "\n")) + 2
}

func (m statusWorkspaceTargetModel) RepositoryURL() string {
	if m.ghActionOrganization != nil || m.ghActionRepository != nil {
		return "https://github.com/" + *m.ghActionOrganization + "/" + *m.ghActionRepository
	}

	return ""
}

func (m statusWorkspaceTargetModel) Success() bool {
	return m.workspaceEventCompilation.success
}

func (m statusWorkspaceTargetModel) TargetHeading() string {
	var result strings.Builder

	result.WriteString(m.TargetName())
	result.WriteString(" [")

	switch {
	case m.generatePublished != nil && *m.generatePublished:
		result.WriteString("Published")
	case m.ghActionOrganization != nil && m.ghActionRepository != nil:
		result.WriteString("Unpublished")
	default:
		result.WriteString("Unconfigured")
	}

	if event := m.workspaceEventCompilation; event != nil && event.publishPackageName != nil && event.publishPackageVersion != nil {
		result.WriteString(": ")
		result.WriteString(*event.publishPackageVersion)
	} else if m.generateConfigPostVersion != nil {
		result.WriteString(": ")
		result.WriteString(*m.generateConfigPostVersion)
	} else if m.generateGenLockPreVersion != nil {
		result.WriteString(": ")
		result.WriteString(*m.generateGenLockPreVersion)
	}

	result.WriteString("]")

	return result.String()
}

func (m statusWorkspaceTargetModel) TargetInfo(ctx context.Context) []string {
	var result []string
	if m.workspaceEventCompilation != nil && !m.workspaceEventCompilation.success {
		var message strings.Builder

		message.WriteString(renderAlertErrorText("✖ Last Generate Failed"))

		if m.workspaceEventCompilation.ghActionRunLink != nil {
			message.WriteString(renderAlertErrorText(": "))
			message.WriteString(renderAlertErrorURL(links.Shorten(ctx, *m.workspaceEventCompilation.ghActionRunLink)))
		}

		result = append(result, message.String())
	}

	if m.upgradeDocumentationURL != nil {
		result = append(result, renderAlertWarningText("⚠ Target Upgrade Available: ")+renderAlertWarningURL(*m.upgradeDocumentationURL))
	}

	if m.workspaceEventCompilation != nil && m.workspaceEventCompilation.publishPackageURL != nil {
		result = append(result, renderInfoText("Publish URL: ")+renderInfoURL(*m.workspaceEventCompilation.publishPackageURL))
	}

	if m.RepositoryURL() != "" {
		result = append(result, renderInfoText("Repository URL: "+renderInfoURL(m.RepositoryURL())))
	}

	result = append(result, renderInfoText("Speakeasy URL: "+renderInfoURL(m.speakeasyURL)))

	if m.workspaceEventCompilation != nil && m.workspaceEventCompilation.publishPackageName != nil {
		result = append(result, renderInfoText("Last Publish: "+m.workspaceEventCompilation.PublishInfo()))
	}

	if m.workspaceEventCompilation != nil {
		result = append(result, renderInfoText("Last Generate: "+m.GenerateInfo()))
	}

	return result
}

func (m statusWorkspaceTargetModel) TargetName() string {
	var result strings.Builder

	result.WriteString(m.generateTarget)
	result.WriteString(" (")

	if m.upgradeDocumentationURL != nil {
		result.WriteString("⚠")
	}

	if m.generateTargetVersion != nil && *m.generateTargetVersion != "" {
		result.WriteString(*m.generateTargetVersion)
	} else {
		result.WriteString("v1")
	}

	result.WriteString(")")

	if m.generateTargetName != nil && !slices.Contains(skipMeaninglessTargetNames, *m.generateTargetName) {
		result.WriteString(": ")
		result.WriteString(*m.generateTargetName)
	}

	return result.String()
}

func (m statusWorkspaceTargetModel) TargetInfoPlain(ctx context.Context) []string {
	var result []string

	if m.workspaceEventCompilation != nil && !m.workspaceEventCompilation.success {
		if m.workspaceEventCompilation.ghActionRunLink != nil {
			result = append(result, "✖ Last Generate Failed: "+links.Shorten(ctx, *m.workspaceEventCompilation.ghActionRunLink))
		} else {
			result = append(result, "✖ Last Generate Failed")
		}
	}

	if m.upgradeDocumentationURL != nil {
		result = append(result, "⚠ Target Upgrade Available: "+*m.upgradeDocumentationURL)
	}

	if m.workspaceEventCompilation != nil && m.workspaceEventCompilation.publishPackageURL != nil {
		result = append(result, "Publish URL: "+*m.workspaceEventCompilation.publishPackageURL)
	}

	if m.RepositoryURL() != "" {
		result = append(result, "Repository URL: "+m.RepositoryURL())
	}

	result = append(result, "Speakeasy URL: "+m.speakeasyURL)

	if m.workspaceEventCompilation != nil && m.workspaceEventCompilation.publishPackageName != nil {
		result = append(result, "Last Publish: "+m.workspaceEventCompilation.PublishInfo())
	}

	if m.workspaceEventCompilation != nil {
		result = append(result, "Last Generate: "+m.GenerateInfo())
	}

	return result
}

func (m statusWorkspaceTargetModel) PrintConsole(ctx context.Context) {
	logger := log.From(ctx)

	logger.Println(m.TargetName())

	for _, line := range m.TargetInfoPlainAligned(ctx) {
		logger.Printf("  %s", line)
	}
}

func (m statusWorkspaceTargetModel) TargetInfoPlainAligned(ctx context.Context) []string {
	var result []string

	// Determine status
	var status string
	switch {
	case m.generatePublished != nil && *m.generatePublished:
		status = "Published"
	case m.ghActionOrganization != nil && m.ghActionRepository != nil:
		status = "Unpublished"
	default:
		status = "Unconfigured"
	}
	result = append(result, fmt.Sprintf("%-17s %s", "Status:", status))

	// Version
	var version string
	if event := m.workspaceEventCompilation; event != nil && event.publishPackageName != nil && event.publishPackageVersion != nil {
		version = *event.publishPackageVersion
	} else if m.generateConfigPostVersion != nil {
		version = *m.generateConfigPostVersion
	} else if m.generateGenLockPreVersion != nil {
		version = *m.generateGenLockPreVersion
	}
	if version != "" {
		result = append(result, fmt.Sprintf("%-17s %s", "Version:", version))
	}

	// Errors/Warnings
	if m.workspaceEventCompilation != nil && !m.workspaceEventCompilation.success {
		if m.workspaceEventCompilation.ghActionRunLink != nil {
			result = append(result, fmt.Sprintf("%-17s %s", "⚠ Generate Failed:", links.Shorten(ctx, *m.workspaceEventCompilation.ghActionRunLink)))
		} else {
			result = append(result, fmt.Sprintf("%-17s %s", "⚠ Generate Failed:", "yes"))
		}
	}

	if m.upgradeDocumentationURL != nil {
		result = append(result, fmt.Sprintf("%-17s %s", "⚠ Upgrade Avail:", *m.upgradeDocumentationURL))
	}

	// URLs
	if m.workspaceEventCompilation != nil && m.workspaceEventCompilation.publishPackageURL != nil {
		result = append(result, fmt.Sprintf("%-17s %s", "Publish URL:", *m.workspaceEventCompilation.publishPackageURL))
	}

	if m.RepositoryURL() != "" {
		result = append(result, fmt.Sprintf("%-17s %s", "Repository URL:", m.RepositoryURL()))
	}

	result = append(result, fmt.Sprintf("%-17s %s", "Speakeasy URL:", m.speakeasyURL))

	// Timestamps
	if m.workspaceEventCompilation != nil && m.workspaceEventCompilation.publishPackageName != nil {
		result = append(result, fmt.Sprintf("%-17s %s", "Last Publish:", m.workspaceEventCompilation.PublishInfo()))
	}

	if m.workspaceEventCompilation != nil {
		result = append(result, fmt.Sprintf("%-17s %s", "Last Generate:", m.GenerateInfo()))
	}

	return result
}

func renderAlertErrorText(s string) string {
	return styles.None.Foreground(styles.Colors.Red).Italic(true).Render(s)
}

func renderAlertErrorURL(s string) string {
	return styles.None.Foreground(styles.Colors.Red).Italic(true).Underline(true).Render(s)
}

func renderAlertWarningText(s string) string {
	return styles.None.Foreground(styles.Colors.Yellow).Italic(true).Render(s)
}

func renderAlertWarningURL(s string) string {
	return styles.None.Foreground(styles.Colors.Yellow).Italic(true).Underline(true).Render(s)
}

func renderInfoText(s string) string {
	return styles.Dimmed.Render(s)
}

func renderInfoURL(s string) string {
	return styles.None.Foreground(styles.Colors.BrightGrey).Underline(true).Render(s)
}

func renderPublishedSuccessTargetBox(width int, heading string, additionalLines ...string) string {
	return renderTargetBox(width, styles.Colors.Green, heading, additionalLines...)
}

func renderPublishedErrorTargetBox(width int, heading string, additionalLines ...string) string {
	return renderTargetBox(width, styles.Colors.Red, heading, additionalLines...)
}

func renderConfiguredTargetBox(width int, heading string, additionalLines ...string) string {
	return renderTargetBox(width, styles.Colors.Yellow, heading, additionalLines...)
}

func renderUnconfiguredTargetBox(width int, heading string, additionalLines ...string) string {
	return renderTargetBox(width, styles.Colors.Blue, heading, additionalLines...)
}

func renderOverviewBox(lines ...string) string {
	s := lipgloss.NewStyle().Foreground(styles.Colors.SpeakeasyPrimary).Bold(true).Render("// SPEAKEASY")

	for _, line := range lines {
		s += "\n" + lipgloss.NewStyle().Foreground(styles.Colors.WhiteBlackAdaptive).Bold(true).Render(line)
	}

	// Leave room for padding (if the terminal is too small to fit, we need to wrap)
	width := min(lipgloss.Width(s)+2, styles.TerminalWidth()-2)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Colors.WhiteBlackAdaptive).
		Padding(0, 1).
		AlignHorizontal(lipgloss.Left).
		Width(width).
		Render(s)
}

func renderTargetBox(width int, color lipgloss.AdaptiveColor, heading string, additionalLines ...string) string {
	s := lipgloss.NewStyle().Foreground(color).Bold(true).Render(utils.CapitalizeFirst(heading))

	for _, line := range additionalLines {
		s += "\n" + lipgloss.NewStyle().Foreground(color).Render(line)
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Padding(0, 1).
		AlignHorizontal(lipgloss.Left).
		Width(width).
		Render(s)
}
