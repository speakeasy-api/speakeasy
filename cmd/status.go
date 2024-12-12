package cmd

import (
	"cmp"
	"context"
	"fmt"
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
	"github.com/speakeasy-api/speakeasy/internal/links"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/sdk"
	"github.com/speakeasy-api/speakeasy/internal/utils"
)

type statusFlagsArgs struct{}

var statusCmd = &model.ExecutableCommand[statusFlagsArgs]{
	Usage:        "status",
	Short:        "Review status of current workspace",
	Run:          runStatus,
	RequiresAuth: true,
	Flags:        []flag.Flag{},
}

func runStatus(ctx context.Context, flags statusFlagsArgs) error {
	client, err := sdk.InitSDK()

	if err != nil {
		return fmt.Errorf("error initializing Speakeasy client: %w", err)
	}

	model, err := newStatusModel(ctx, client)

	if err != nil {
		return err
	}

	model.Print(ctx)

	return nil
}

var (
	skipMeaninglessTargetNames = []string{
		"",
		"first-target",
		"my-first-target",
	}
)

type statusModel struct {
	organization statusOrganizationModel
	workspace    statusWorkspaceModel
}

func newStatusModel(ctx context.Context, client *speakeasyclientsdkgo.Speakeasy) (statusModel, error) {
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

	if wsRes.StatusCode != 200 {
		return result, fmt.Errorf("unexpected status code getting Speakeasy workspace: %d", wsRes.StatusCode)
	}

	if wsRes.Workspace == nil {
		return result, fmt.Errorf("unexpected missing workspace response")
	}

	orgReq := operations.GetOrganizationRequest{
		OrganizationID: wsRes.Workspace.OrganizationID,
	}

	orgRes, err := client.Organizations.Get(ctx, orgReq)

	if err != nil {
		return result, fmt.Errorf("error getting Speakeasy organization: %w", err)
	}

	if orgRes.StatusCode != 200 {
		return result, fmt.Errorf("unexpected status code getting Speakeasy organization: %d", orgRes.StatusCode)
	}

	if orgRes.Organization == nil {
		return result, fmt.Errorf("unexpected missing organization response")
	}

	organization, err := newStatusOrganizationModel(ctx, client, *orgRes.Organization)

	if err != nil {
		return result, err
	}

	result.organization = organization

	workspace, err := newStatusWorkspaceModel(ctx, client, result.organization, *wsRes.Workspace)

	if err != nil {
		return result, err
	}

	result.workspace = workspace

	return result, nil
}

func (m statusModel) Print(ctx context.Context) {
	logger := log.From(ctx)

	var overviewLines []string

	overviewLines = append(overviewLines, fmt.Sprintf("Workspace: %s/%s", m.organization.Name(), m.workspace.Name()))

	var accountTypeLine strings.Builder

	accountTypeLine.WriteString("Account Type: ")
	accountTypeLine.WriteString(m.organization.accountType)

	if m.organization.freeTrialExpiry != nil {
		expiryDiff := time.Until(*m.organization.freeTrialExpiry)
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

	overviewLines = append(overviewLines, accountTypeLine.String())

	logger.Println(renderOverviewBox(overviewLines...))

	m.workspace.targets.Print(ctx)
}

type statusOrganizationModel struct {
	accountType     string
	freeTrialExpiry *time.Time
	name            string
	slug            string
}

func newStatusOrganizationModel(ctx context.Context, _ *speakeasyclientsdkgo.Speakeasy, organization shared.Organization) (statusOrganizationModel, error) {
	result := statusOrganizationModel{
		accountType:     string(organization.AccountType),
		freeTrialExpiry: organization.FreeTrialExpiry,
		name:            organization.Name,
		slug:            organization.Slug,
	}

	return result, nil
}

func (m statusOrganizationModel) Name() string {
	if m.name != "" {
		return m.name
	}

	return m.slug
}

type statusWorkspaceModel struct {
	name           string
	id             string
	slug           string
	organizationID string
	targets        statusWorkspaceTargetsModel
}

func newStatusWorkspaceModel(ctx context.Context, client *speakeasyclientsdkgo.Speakeasy, org statusOrganizationModel, workspace shared.Workspace) (statusWorkspaceModel, error) {
	result := statusWorkspaceModel{
		id:             workspace.ID,
		name:           workspace.Name,
		organizationID: workspace.OrganizationID,
		slug:           workspace.Slug,
	}

	wsTargetsreq := operations.GetWorkspaceTargetsRequest{}

	wsTargetsRes, err := client.Events.GetTargets(ctx, wsTargetsreq)

	if err != nil {
		return result, fmt.Errorf("error getting Speakeasy workspace targets: %w", err)
	}

	if wsTargetsRes.StatusCode != 200 {
		return result, fmt.Errorf("unexpected status code getting Speakeasy workspace targets: %d", wsTargetsRes.StatusCode)
	}

	targets, err := newStatusWorkspaceTargetsModel(ctx, client, org, result, wsTargetsRes.TargetSDKList)

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

func newStatusWorkspaceTargetsModel(ctx context.Context, client *speakeasyclientsdkgo.Speakeasy, org statusOrganizationModel, workspace statusWorkspaceModel, targets []shared.TargetSDK) (statusWorkspaceTargetsModel, error) {
	var result statusWorkspaceTargetsModel

	targetModels := make([]*statusWorkspaceTargetModel, len(targets))
	eg, ctx := errgroup.WithContext(ctx)

	for index, target := range targets {
		// Archived
		if target.LastEventInteractionType == shared.InteractionTypeTombstone {
			continue
		}

		eg.Go(func() error {
			targetModel, err := newStatusWorkspaceTargetModel(ctx, client, org, workspace, target)

			if err != nil {
				return err
			}

			targetModels[index] = targetModel

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
	success                          bool
}

func newStatusWorkspaceEventModel(event shared.CliEvent) *statusWorkspaceEventModel {
	result := &statusWorkspaceEventModel{
		continuousIntegrationEnvironment: event.ContinuousIntegrationEnvironment,
		createdAt:                        event.CreatedAt,
		ghActionRunLink:                  event.GhActionRunLink,
		gitUserEmail:                     event.GitUserEmail,
		gitUserName:                      event.GitUserName,
		hostname:                         event.Hostname,
		publishPackageName:               event.PublishPackageName,
		publishPackageRegistryName:       event.PublishPackageRegistryName,
		publishPackageURL:                event.PublishPackageURL,
		publishPackageVersion:            event.PublishPackageVersion,
		success:                          event.Success,
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

	if m.ghActionRunLink != nil {
		result.WriteString("GitHub Actions")
	} else if m.continuousIntegrationEnvironment != nil {
		result.WriteString("CI")
	} else if m.gitUserName != nil {
		result.WriteString(*m.gitUserName)
	} else if m.gitUserEmail != nil {
		result.WriteString(*m.gitUserEmail)
	} else if m.hostname != nil {
		result.WriteString(*m.hostname)
	} else {
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

	result.WriteString(m.createdAt.Format(time.RFC3339))
	result.WriteString(" by ")

	if m.ghActionRunLink != nil {
		result.WriteString("GitHub Actions")
	} else if m.continuousIntegrationEnvironment != nil {
		result.WriteString("CI")
	} else if m.gitUserName != nil {
		result.WriteString(*m.gitUserName)
	} else if m.gitUserEmail != nil {
		result.WriteString(*m.gitUserEmail)
	} else if m.hostname != nil {
		result.WriteString(*m.hostname)
	} else {
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

	// Generate events
	generateLastEvent        *statusWorkspaceEventModel
	generateLastSuccessEvent *statusWorkspaceEventModel

	// Publish events
	publishLastEvent        *statusWorkspaceEventModel
	publishLastSuccessEvent *statusWorkspaceEventModel

	// Speakeasy URL
	speakeasyURL string

	// If a target upgrade is recommended, the URL for upgrade documentation.
	upgradeDocumentationURL *string
}

func newStatusWorkspaceTargetModel(ctx context.Context, client *speakeasyclientsdkgo.Speakeasy, org statusOrganizationModel, workspace statusWorkspaceModel, target shared.TargetSDK) (*statusWorkspaceTargetModel, error) {
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

	lastGenerateEvent, err := searchWorkspaceTargetLastEvent(ctx, client, workspace.id, target.ID, shared.InteractionTypeTargetGenerate, false)

	if err != nil {
		return result, fmt.Errorf("error searching last Speakeasy target generate event: %w", err)
	}

	if lastGenerateEvent != nil {
		result.generateLastEvent = newStatusWorkspaceEventModel(*lastGenerateEvent)

		if lastGenerateEvent.Success {
			result.generateLastSuccessEvent = result.generateLastEvent
		}
	}

	if result.generateLastSuccessEvent == nil {
		lastGenerateSuccessEvent, err := searchWorkspaceTargetLastEvent(ctx, client, workspace.id, target.ID, shared.InteractionTypeTargetGenerate, true)

		if err != nil {
			return result, fmt.Errorf("error searching last Speakeasy target generate success event: %w", err)
		}

		if lastGenerateSuccessEvent != nil {
			result.generateLastSuccessEvent = newStatusWorkspaceEventModel(*lastGenerateSuccessEvent)
		}
	}

	lastPublishEvent, err := searchWorkspaceTargetLastEvent(ctx, client, workspace.id, target.ID, shared.InteractionTypePublish, false)

	if err != nil {
		return result, fmt.Errorf("error searching last Speakeasy target publish event: %w", err)
	}

	if lastPublishEvent != nil {
		result.publishLastEvent = newStatusWorkspaceEventModel(*lastPublishEvent)

		if lastPublishEvent.Success {
			result.publishLastSuccessEvent = result.publishLastEvent
		}
	}

	if result.publishLastSuccessEvent == nil {
		lastPublishSuccessEvent, err := searchWorkspaceTargetLastEvent(ctx, client, workspace.id, target.ID, shared.InteractionTypePublish, true)

		if err != nil {
			return result, fmt.Errorf("error searching last Speakeasy target publish success event: %w", err)
		}

		if lastPublishSuccessEvent != nil {
			result.publishLastSuccessEvent = newStatusWorkspaceEventModel(*lastPublishSuccessEvent)
		}
	}

	return result, nil
}

func (m statusWorkspaceTargetModel) GenerateInfo() string {
	var result strings.Builder

	if m.ghActionRunLink == nil {
		result.WriteString("locally at ")
	}

	result.WriteString(m.lastEventCreatedAt.Format(time.RFC3339))
	result.WriteString(" by ")

	if m.ghActionRunLink != nil {
		result.WriteString("GitHub Actions")
	} else if m.continuousIntegrationEnvironment != nil {
		result.WriteString("CI")
	} else if m.gitUserName != nil {
		result.WriteString(*m.gitUserName)
	} else if m.gitUserEmail != nil {
		result.WriteString(*m.gitUserEmail)
	} else if m.hostname != nil {
		result.WriteString(*m.hostname)
	} else {
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
	if event := m.publishLastEvent; event != nil && !event.success {
		return false
	}

	if event := m.generateLastEvent; event != nil && !event.success {
		return false
	}

	return true
}

func (m statusWorkspaceTargetModel) TargetHeading() string {
	var result strings.Builder

	result.WriteString(m.TargetName())
	result.WriteString(" [")

	if m.generatePublished != nil && *m.generatePublished {
		result.WriteString("Published")
	} else if m.ghActionOrganization != nil && m.ghActionRepository != nil {
		result.WriteString("Unpublished")
	} else {
		result.WriteString("Unconfigured")
	}

	if event := m.publishLastSuccessEvent; event != nil && event.publishPackageName != nil && event.publishPackageVersion != nil {
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

	if m.publishLastEvent != nil && !m.publishLastEvent.success {
		var message strings.Builder

		message.WriteString(renderAlertErrorText("✖ Last Publish Failed"))

		if m.publishLastEvent.ghActionRunLink != nil {
			message.WriteString(renderAlertErrorText(": "))
			message.WriteString(renderAlertErrorURL(links.Shorten(ctx, *m.publishLastEvent.ghActionRunLink)))
		}

		result = append(result, message.String())
	}

	if m.generateLastEvent != nil && !m.generateLastEvent.success {
		var message strings.Builder

		message.WriteString(renderAlertErrorText("✖ Last Generate Failed"))

		if m.generateLastEvent.ghActionRunLink != nil {
			message.WriteString(renderAlertErrorText(": "))
			message.WriteString(renderAlertErrorURL(links.Shorten(ctx, *m.generateLastEvent.ghActionRunLink)))
		}

		result = append(result, message.String())
	}

	if m.upgradeDocumentationURL != nil {
		result = append(result, renderAlertWarningText("⚠ Target Upgrade Available: ")+renderAlertWarningURL(*m.upgradeDocumentationURL))
	}

	if m.publishLastSuccessEvent != nil && m.publishLastSuccessEvent.publishPackageURL != nil {
		result = append(result, renderInfoText("Publish URL: ")+renderInfoURL(*m.publishLastSuccessEvent.publishPackageURL))
	}

	if m.RepositoryURL() != "" {
		result = append(result, renderInfoText("Repository URL: "+renderInfoURL(m.RepositoryURL())))
	}

	result = append(result, renderInfoText("Speakeasy URL: "+renderInfoURL(m.speakeasyURL)))

	if m.publishLastEvent != nil {
		if !m.publishLastEvent.success {
			result = append(result, renderInfoText("Last Publish Attempt: "+m.publishLastEvent.PublishInfo()))

			if m.publishLastSuccessEvent != nil {
				result = append(result, renderInfoText("Last Publish Success: "+m.publishLastSuccessEvent.GenerateInfo()))
			} else {
				result = append(result, renderInfoText("Last Publish Success: Unknown"))
			}
		} else {
			result = append(result, renderInfoText("Last Publish: "+m.publishLastEvent.PublishInfo()))
		}
	}

	if m.generateLastEvent != nil {
		if !m.generateLastEvent.success {
			result = append(result, renderInfoText("Last Generate Attempt: "+m.generateLastEvent.GenerateInfo()))

			if m.generateLastSuccessEvent != nil {
				result = append(result, renderInfoText("Last Generate Success: "+m.generateLastSuccessEvent.GenerateInfo()))
			} else {
				result = append(result, renderInfoText("Last Generate Success: Unknown"))
			}
		} else {
			result = append(result, renderInfoText("Last Generate: "+m.generateLastEvent.GenerateInfo()))
		}
	} else {
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

func searchWorkspaceTargetLastEvent(ctx context.Context, client *speakeasyclientsdkgo.Speakeasy, workspaceID string, targetID string, interactionType shared.InteractionType, successOnly bool) (*shared.CliEvent, error) {
	limit := int64(1)
	req := operations.SearchWorkspaceEventsRequest{
		GenerateGenLockID: &targetID,
		InteractionType:   &interactionType,
		Limit:             &limit,
		WorkspaceID:       &workspaceID,
	}

	if successOnly {
		req.Success = &successOnly
	}

	res, err := client.Events.Search(ctx, req)

	if err != nil {
		return nil, fmt.Errorf("error calling Speakeasy API: %w", err)
	}

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("expected status code 200, got: %d", res.StatusCode)
	}

	if len(res.CliEventBatch) == 0 {
		return nil, nil
	}

	if len(res.CliEventBatch) > 1 {
		return nil, fmt.Errorf("expected at most one event, got: %d", len(res.CliEventBatch))
	}

	return &res.CliEventBatch[0], nil
}
