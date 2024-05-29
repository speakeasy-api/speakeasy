package cmd

import (
	"context"
	"fmt"
	"strings"

	core "github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/registry"
)

type tagFlagsArgs struct {
	NamespaceName  string   `json:"namespace-name"`
	RevisionDigest string   `json:"revision-digest"`
	Tags           []string `json:"tags"`
}

var tagCmd = &model.ExecutableCommand[tagFlagsArgs]{
	Usage:        "tag",
	Short:        "Add tags to a given revision of your API. Specific to a registry namespace",
	Run:          runTag,
	RequiresAuth: true,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "namespace-name",
			Shorthand:   "n",
			Description: "the revision to tag",
			Required:    true,
		},
		flag.StringFlag{
			Name:        "revision-digest",
			Shorthand:   "r",
			Description: "the revision ID to tag",
			Required:    true,
		},
		flag.StringSliceFlag{
			Name:        "tags",
			Shorthand:   "t",
			Description: "A list of tags to apply",
			Required:    true,
		},
	},
}

func runTag(ctx context.Context, flags tagFlagsArgs) error {
	workspaceID, _ := core.GetWorkspaceIDFromContext(ctx)
	if !registry.IsRegistryEnabled(ctx) {
		return fmt.Errorf("API Registry is not enabled for this workspace %s", workspaceID)
	}
	revisionDigest := flags.RevisionDigest
	if !strings.HasPrefix(revisionDigest, "sha256:") {
		revisionDigest = "sha256:" + revisionDigest
	}

	err := registry.AddTags(ctx, flags.NamespaceName, revisionDigest, flags.Tags)
	if err == nil {
		formattedTags := []string{
			"Tags:",
		}
		for _, tag := range flags.Tags {
			formattedTags = append(formattedTags, styles.DimmedItalic.Render(fmt.Sprintf("- %s", tag)))
		}

		msg := styles.RenderInstructionalMessage(
			fmt.Sprintf("Tags successfully added to %s@%s", flags.NamespaceName, revisionDigest),
			formattedTags...)
		log.From(ctx).Println(msg)
	}

	return err
}
