package reports

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy/internal/links"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/sdk"
	"github.com/stoewer/go-strcase"
	"os"
	"path/filepath"
)

type ReportResult struct {
	Message   string
	URL       string
	LocalPath string
	Digest    string
	Type      shared.Type
}

func UploadReport(ctx context.Context, reportBytes []byte, reportType shared.Type) (ReportResult, error) {
	md5Hasher := md5.New()
	if _, err := md5Hasher.Write(reportBytes); err != nil {
		return writeLocally("", reportBytes, reportType)
	}
	digest := hex.EncodeToString(md5Hasher.Sum(nil))

	s, err := sdk.InitSDK()
	if err != nil {
		return writeLocally(digest, reportBytes, reportType)
	}

	uploadRes, err := s.Reports.UploadReport(ctx, operations.UploadReportRequestBody{
		Data: shared.Report{
			Type: reportType.ToPointer(),
		},
		File: operations.File{
			Content:  reportBytes,
			FileName: digest + ".html",
		},
	})
	if err != nil {
		log.From(ctx).Warnf("Failed to upload report to Speakeasy %s", err.Error())
		return writeLocally(digest, reportBytes, reportType)
	}

	cliEvent := events.GetTelemetryEventFromContext(ctx)
	if cliEvent != nil {
		switch reportType {
		case shared.TypeLinting:
			cliEvent.LintReportDigest = &digest
		case shared.TypeChanges:
			cliEvent.OpenapiDiffReportDigest = &digest
		}
	}

	url := uploadRes.UploadedReport.GetURL()
	url = links.Shorten(ctx, url)

	return ReportResult{
		Message: fmt.Sprintf("%s available to view at: %s", ReportTitle(reportType), url),
		URL:     url,
		Digest:  digest,
		Type:    reportType,
	}, nil
}

func writeLocally(digest string, reportBytes []byte, reportType shared.Type) (r ReportResult, err error) {
	baseDir, err := os.UserHomeDir()
	if err != nil {
		baseDir = os.TempDir()
	}

	outputDir := filepath.Join(baseDir, ".speakeasy", "temp")

	err = os.MkdirAll(outputDir, os.ModePerm)
	if err != nil {
		return
	}

	uniqueFilename := digest
	filenamePrefix := strcase.KebabCase(ReportTitle(reportType))

	// "*" will be replaced with a random string
	rf, err := os.CreateTemp(outputDir, fmt.Sprintf("%s-%s-*.html", filenamePrefix, uniqueFilename))
	if err != nil {
		return
	}
	defer rf.Close()

	if _, err = rf.Write(reportBytes); err != nil {
		return
	}

	r.Message = fmt.Sprintf("%s written to: %s", ReportTitle(reportType), rf.Name())
	r.LocalPath = rf.Name()
	r.Type = reportType

	return
}

func (r ReportResult) Location() string {
	if r.URL != "" {
		return r.URL
	}

	return r.LocalPath
}

func (r ReportResult) Title() string {
	return ReportTitle(r.Type)
}

func ReportTitle(reportType shared.Type) string {
	switch reportType {
	case shared.TypeLinting:
		return "Lint report"
	case shared.TypeChanges:
		return "API Change report"
	}

	return "Report"
}
