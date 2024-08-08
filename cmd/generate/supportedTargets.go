package generate

import (
	"context"
	"fmt"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
)

type supportedTargetsFlags struct{}

var suportedTargetsCmd = &model.ExecutableCommand[supportedTargetsFlags]{
	Usage:  "supported-targets",
	Short:  "Returns a list of supported generation targets.",
	Run:    runSupportedTargets,
	Flags:  []flag.Flag{},
	Hidden: true,
}

func runSupportedTargets(ctx context.Context, flags supportedTargetsFlags) error {
	fmt.Println(strings.Join(SDKSupportedLanguageTargets(), ","))
	return nil
}
