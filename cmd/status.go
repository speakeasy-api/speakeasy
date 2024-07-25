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

	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
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
	accountType  *string
	organization statusOrganizationModel
	workspace    statusWorkspaceModel
}

func newStatusModel(ctx context.Context, client *speakeasyclientsdkgo.Speakeasy) (statusModel, error) {
	var model statusModel

	if accountType := core.GetAccountTypeFromContext(ctx); accountType != nil {
		v := string(*accountType)
		model.accountType = &v
	}

	workspaceID, err := core.GetWorkspaceIDFromContext(ctx)

	if err != nil {
		return model, err
	}

	model.workspace.id = workspaceID

	model.organization.slug = core.GetOrgSlugFromContext(ctx)
	model.workspace.slug = core.GetWorkspaceSlugFromContext(ctx)

	req := operations.GetWorkspaceTargetsRequest{
		WorkspaceID: &model.workspace.id,
	}

	res, err := client.Events.GetWorkspaceTargets(ctx, req)

	if err != nil {
		return model, fmt.Errorf("error getting Speakeasy workspace targets: %w", err)
	}

	if res.StatusCode != 200 {
		return model, fmt.Errorf("unexpected status code getting Speakeasy workspace targets: %d", res.StatusCode)
	}

	model.workspace.targets, err = newStatusWorkspaceTargetsModel(ctx, client, model.organization, model.workspace, res.TargetSDKList)

	return model, err
}

func (m statusModel) Print(ctx context.Context) {
	m.workspace.targets.Print(ctx)
}

type statusOrganizationModel struct {
	slug string
}

type statusWorkspaceModel struct {
	id      string
	slug    string
	targets statusWorkspaceTargetsModel
}

type statusWorkspaceTargetsModel struct {
	published    []statusWorkspaceTargetModel
	configured   []statusWorkspaceTargetModel
	unconfigured []statusWorkspaceTargetModel
}

func newStatusWorkspaceTargetsModel(ctx context.Context, client *speakeasyclientsdkgo.Speakeasy, org statusOrganizationModel, workspace statusWorkspaceModel, targets []shared.TargetSDK) (statusWorkspaceTargetsModel, error) {
	var result statusWorkspaceTargetsModel

	for _, target := range targets {
		targetModel, err := newStatusWorkspaceTargetModel(ctx, client, org, workspace, target)

		if err != nil {
			return result, err
		}

		if target.GeneratePublished != nil && *target.GeneratePublished {
			result.published = append(result.published, targetModel)

			continue
		}

		if target.GhActionOrganization != nil && target.GhActionRepository != nil {
			result.configured = append(result.configured, targetModel)

			continue
		}

		if target.GenerateConfigPostVersion == nil {
			// Archived
			continue
		}

		result.unconfigured = append(result.unconfigured, targetModel)
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

func newStatusWorkspaceEventModel(event shared.CliEvent) statusWorkspaceEventModel {
	result := statusWorkspaceEventModel{
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
	generateEvents []statusWorkspaceEventModel

	// Publish events
	publishEvents []statusWorkspaceEventModel

	// Speakeasy URL
	speakeasyURL string

	// If a target upgrade is recommended, the URL for upgrade documentation.
	upgradeDocumentationURL *string
}

func newStatusWorkspaceTargetModel(ctx context.Context, client *speakeasyclientsdkgo.Speakeasy, org statusOrganizationModel, workspace statusWorkspaceModel, target shared.TargetSDK) (statusWorkspaceTargetModel, error) {
	result := statusWorkspaceTargetModel{
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
		speakeasyURL:                      links.Shorten(ctx, fmt.Sprintf("https://app.speakeasyapi.dev/org/%s/%s/targets/%s", org.slug, workspace.slug, target.ID)),
		success:                           target.Success,
	}

	if result.generateTarget == "python" && (result.generateTargetVersion == nil || *result.generateTargetVersion == "v1") {
		upgradeURL := "https://www.speakeasy.com/docs/python-migration"
		result.upgradeDocumentationURL = &upgradeURL
	}

	interactionTypeTargetGenerate := shared.InteractionTypeTargetGenerate
	req := operations.SearchWorkspaceEventsRequest{
		GenerateGenLockID: &target.ID,
		InteractionType:   &interactionTypeTargetGenerate,
		WorkspaceID:       &workspace.id,
	}

	res, err := client.Events.SearchWorkspaceEvents(ctx, req)

	if err != nil {
		return result, fmt.Errorf("error searching Speakeasy target events: %w", err)
	}

	if res.StatusCode != 200 {
		return result, fmt.Errorf("unexpected status code searching Speakeasy target events: %d", res.StatusCode)
	}

	if len(res.CliEventBatch) == 0 {
		return result, nil
	}

	for _, event := range res.CliEventBatch {
		eventModel := newStatusWorkspaceEventModel(event)
		result.generateEvents = append(result.generateEvents, eventModel)
	}

	interactionTypePublish := shared.InteractionTypePublish
	req = operations.SearchWorkspaceEventsRequest{
		GenerateGenLockID: &target.ID,
		InteractionType:   &interactionTypePublish,
		WorkspaceID:       &workspace.id,
	}

	res, err = client.Events.SearchWorkspaceEvents(ctx, req)

	if err != nil {
		return result, fmt.Errorf("error searching Speakeasy target events: %w", err)
	}

	if res.StatusCode != 200 {
		return result, fmt.Errorf("unexpected status code searching Speakeasy target events: %d", res.StatusCode)
	}

	if len(res.CliEventBatch) == 0 {
		return result, nil
	}

	for _, event := range res.CliEventBatch {
		eventModel := newStatusWorkspaceEventModel(event)
		result.publishEvents = append(result.publishEvents, eventModel)
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

func (m statusWorkspaceTargetModel) LastGenerateEvent() *statusWorkspaceEventModel {
	if len(m.generateEvents) == 0 {
		return nil
	}

	return &m.generateEvents[0]
}

func (m statusWorkspaceTargetModel) LastGenerateSuccessEvent() *statusWorkspaceEventModel {
	if len(m.generateEvents) == 0 {
		return nil
	}

	for _, event := range m.generateEvents {
		if event.success {
			return &event
		}
	}

	return nil
}

func (m statusWorkspaceTargetModel) LastPublishEvent() *statusWorkspaceEventModel {
	if len(m.publishEvents) == 0 {
		return nil
	}

	return &m.publishEvents[0]
}

func (m statusWorkspaceTargetModel) LastPublishSuccessEvent() *statusWorkspaceEventModel {
	if len(m.publishEvents) == 0 {
		return nil
	}

	for _, event := range m.publishEvents {
		if event.success {
			return &event
		}
	}

	return nil
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
	if event := m.LastPublishEvent(); event != nil && !event.success {
		return false
	}

	if event := m.LastGenerateEvent(); event != nil && !event.success {
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

	if event := m.LastPublishSuccessEvent(); event != nil && event.publishPackageName != nil && event.publishPackageVersion != nil {
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
	lastGenerateEvent := m.LastGenerateEvent()
	lastGenerateSuccessEvent := m.LastGenerateSuccessEvent()
	lastPublishEvent := m.LastPublishEvent()
	lastPublishSuccessEvent := m.LastPublishSuccessEvent()

	var result []string

	if lastPublishEvent != nil && !lastPublishEvent.success {
		var message strings.Builder

		message.WriteString("✖ Last Publish Failed")

		if lastPublishEvent.ghActionRunLink != nil {
			message.WriteString(": ")
			message.WriteString(links.Shorten(ctx, *lastPublishEvent.ghActionRunLink))
		}

		result = append(result, styles.None.Foreground(styles.Colors.Red).Italic(true).Render(message.String()))
	}

	if lastGenerateEvent != nil && !lastGenerateEvent.success {
		var message strings.Builder

		message.WriteString("✖ Last Generate Failed")

		if lastGenerateEvent.ghActionRunLink != nil {
			message.WriteString(": ")
			message.WriteString(links.Shorten(ctx, *lastGenerateEvent.ghActionRunLink))
		}

		result = append(result, styles.None.Foreground(styles.Colors.Red).Italic(true).Render(message.String()))
	}

	if m.upgradeDocumentationURL != nil {
		result = append(result, styles.None.Foreground(styles.Colors.Yellow).Italic(true).Render("⚠ Target Upgrade Available: "+*m.upgradeDocumentationURL))
	}

	if lastPublishSuccessEvent != nil && lastPublishSuccessEvent.publishPackageURL != nil {
		result = append(result, styles.Dimmed.Render("Publish URL: "+*lastPublishSuccessEvent.publishPackageURL))
	}

	if m.RepositoryURL() != "" {
		result = append(result, styles.Dimmed.Render("Repository URL: "+m.RepositoryURL()))
	}

	result = append(result, styles.Dimmed.Render("Speakeasy URL: "+m.speakeasyURL))

	if lastPublishEvent != nil {
		if !lastPublishEvent.success {
			result = append(result, styles.Dimmed.Render("Last Publish Attempt: "+lastPublishEvent.PublishInfo()))

			if lastPublishSuccessEvent != nil {
				result = append(result, styles.Dimmed.Render("Last Publish Success: "+lastPublishSuccessEvent.GenerateInfo()))
			} else {
				result = append(result, styles.Dimmed.Render("Last Publish Success: Unknown"))
			}
		} else {
			result = append(result, styles.Dimmed.Render("Last Publish: "+lastPublishEvent.PublishInfo()))
		}
	}

	if lastGenerateEvent != nil {
		if !lastGenerateEvent.success {
			result = append(result, styles.Dimmed.Render("Last Generate Attempt: "+lastGenerateEvent.GenerateInfo()))

			if lastGenerateSuccessEvent != nil {
				result = append(result, styles.Dimmed.Render("Last Generate Success: "+lastGenerateSuccessEvent.GenerateInfo()))
			} else {
				result = append(result, styles.Dimmed.Render("Last Generate Success: Unknown"))
			}
		} else {
			result = append(result, styles.Dimmed.Render("Last Generate: "+lastGenerateEvent.GenerateInfo()))
		}
	} else {
		result = append(result, styles.Dimmed.Render("Last Generate: "+m.GenerateInfo()))
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
