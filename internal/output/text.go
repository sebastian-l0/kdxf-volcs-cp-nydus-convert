package output

import (
	"fmt"
	"io"

	"github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/app"
)

func WriteText(w io.Writer, result *app.Result) error {
	for _, item := range result.Items {
		switch item.Status {
		case "dry_run":
			_, _ = fmt.Fprintf(w, "[DRY-RUN] line=%d repo=%s tag=%s workspace=%s pipeline=%s name=%s imageToConvert=%s\n", item.Line, item.Repository, item.Tag, item.WorkspaceID, item.PipelineID, item.PipelineName, item.ImageToConvert)
		case "triggered":
			_, _ = fmt.Fprintf(w, "[OK] line=%d repo=%s tag=%s workspace=%s pipeline=%s name=%s run_id=%s rate_limit_wait_ms=%d\n", item.Line, item.Repository, item.Tag, item.WorkspaceID, item.PipelineID, item.PipelineName, item.RunID, item.RateLimitWaitMS)
		case "failed":
			code, msg := "", ""
			if item.Error != nil {
				code = item.Error.Code
				msg = item.Error.Message
			}
			_, _ = fmt.Fprintf(w, "[FAIL] line=%d imageToConvert=%s error=%s message=%q\n", item.Line, item.ImageToConvert, code, msg)
		}
	}
	_, err := fmt.Fprintf(w, "Summary: total=%d success=%d failed=%d skipped=%d run_pipeline_calls=%d run_pipeline_qpm=%d dry_run=%t\n", result.Summary.Total, result.Summary.Success, result.Summary.Failed, result.Summary.Skipped, result.Summary.RunPipelineCalls, result.Summary.RunPipelineQPM, result.DryRun)
	return err
}
