package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/app"
	apperrors "github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/errors"
)

func sampleResult() *app.Result {
	return &app.Result{DryRun: true, Summary: app.Summary{Total: 1, Success: 1, RunPipelineQPM: 100}, Items: []app.ResultItem{{Line: 1, ImageToConvert: "registry/ns/repo:v1", Repository: "repo", Tag: "v1", WorkspaceID: "w", PipelineID: "p", Status: "dry_run", DynamicVars: map[string]string{"imageToConvert": "registry/ns/repo:v1", "tag": "v1"}}}}
}

func TestWriteJSON(t *testing.T) {
	var b bytes.Buffer
	if err := Write(&b, "json", sampleResult()); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["dry_run"] != true {
		t.Fatalf("json=%s", b.String())
	}
}

func TestWriteText(t *testing.T) {
	var b bytes.Buffer
	if err := Write(&b, "text", sampleResult()); err != nil {
		t.Fatal(err)
	}
	s := b.String()
	if !strings.Contains(s, "[DRY-RUN]") || !strings.Contains(s, "Summary:") {
		t.Fatalf("text=%s", s)
	}
}

func TestWriteInvalidFormat(t *testing.T) {
	if err := Write(&bytes.Buffer{}, "xml", sampleResult()); apperrors.CodeOf(err) != apperrors.CodeInvalidConfig {
		t.Fatalf("code=%q err=%v", apperrors.CodeOf(err), err)
	}
}
