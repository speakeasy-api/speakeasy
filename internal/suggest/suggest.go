package suggest

import (
	"context"
	"fmt"
	"github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/speakeasy-api/speakeasy-core/suggestions"
	"gopkg.in/yaml.v3"

	"github.com/charmbracelet/lipgloss"
	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy/internal/log"
	overlayUtil "github.com/speakeasy-api/speakeasy/internal/overlay"
	"github.com/speakeasy-api/speakeasy/internal/schema"

	speakeasy "github.com/speakeasy-api/speakeasy-client-sdk-go/v3"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/internal/sdk"
)

func SuggestOperationIDsAndWrite(ctx context.Context, schemaLocation string, asOverlay, yamlOut bool, w io.Writer) error {
	if asOverlay {
		yamlOut = true
	}

	schemaBytes, _, model, err := schema.LoadDocument(ctx, schemaLocation)
	if err != nil {
		return err
	}

	stopSpinner := interactivity.StartSpinner("Generating suggestions...")

	overlay, err := SuggestOperationIDs(ctx, schemaBytes, schemaLocation)

	stopSpinner()

	if err != nil {
		return err
	}

	printSuggestions(ctx, overlay)

	/*
	 * Write the new document or overlay
	 */
	if asOverlay {
		if err := overlay.Format(w); err != nil {
			return err
		}
	} else {
		root := model.Index.GetRootNode()
		if err := overlay.ApplyTo(root); err != nil {
			return err
		}

		finalBytes, err := overlayUtil.Render(root, schemaLocation, yamlOut)
		if err != nil {
			return err
		}

		if _, err = w.Write(finalBytes); err != nil {
			return err
		}
	}

	return nil
}

func SuggestOperationIDs(ctx context.Context, schema []byte, schemaPath string) (*overlay.Overlay, error) {
	summary, err := openapi.GetOASSummary(schema, schemaPath)
	if err != nil || summary == nil {
		return nil, fmt.Errorf("failed to get OAS summary: %w", err)
	}

	diagnosis, err := suggestions.Diagnose(ctx, *summary)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Timeout: 5 * time.Minute}
	severURL := speakeasy.ServerList[speakeasy.ServerProd]
	if strings.Contains(auth.GetServerURL(), "localhost") {
		severURL = "http://localhost:35291"
	}
	client, err := sdk.InitSDK(
		speakeasy.WithClient(httpClient),
		speakeasy.WithServerURL(severURL),
	)
	if err != nil {
		return nil, err
	}

	/* Get suggestion */
	res, err := client.Suggest.Suggest(ctx, operations.SuggestRequest{
		SuggestRequestBody: shared.SuggestRequestBody{
			SuggestionType: shared.SuggestRequestBodySuggestionTypeMethodNames,
			OasSummary:     utils.ConvertOASSummary(*summary),
			Diagnostics:    utils.ConvertDiagnosis(diagnosis),
		},
	})
	if err != nil || res.Schema == nil {
		return nil, err
	}

	bytes, err := ioutil.ReadAll(res.Schema)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	var o overlay.Overlay
	if err := yaml.Unmarshal(bytes, &o); err != nil {
		return nil, fmt.Errorf("error unmarshalling response body into overlay: %w", err)
	}

	return &o, nil
}

var changedStyle = styles.Dimmed.Strikethrough(true)

func printSuggestions(ctx context.Context, overlay *overlay.Overlay) {
	logger := log.From(ctx)

	maxWidth := 0

	var lhs []string
	var rhs []string

	for _, action := range overlay.Actions {
		modification := suggestions.GetModificationExtension(action)
		if modification == nil {
			continue
		}

		before := changedStyle.Render(modification.Before)
		after := styles.Success.Render(modification.After)

		if modification.Before == modification.After {
			before = styles.Info.Render(modification.Before)
			after = styles.DimmedItalic.Render(modification.After)
		}

		lhs = append(lhs, before)
		rhs = append(rhs, after)

		if w := lipgloss.Width(before); w > maxWidth {
			maxWidth = w
		}
	}

	lhsHeading := styles.Info.Width(maxWidth).Underline(true).Render("Original")
	rhsHeading := styles.Success.Underline(true).Render("Suggested")
	logger.Printf("%s    %s", lhsHeading, rhsHeading)

	arrow := styles.HeavilyEmphasized.Render("->")
	for i := range lhs {
		l := lipgloss.NewStyle().Width(maxWidth).Render(strings.TrimSpace(lhs[i]))
		logger.Printf("%s %s %s", l, arrow, strings.TrimSpace(rhs[i]))
	}
}
