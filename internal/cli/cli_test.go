package cli

import (
	"io"
	"testing"

	apperrors "github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/errors"
)

func TestParseRunOptions(t *testing.T) {
	opts, err := ParseRunOptions([]string{"--images-file", "images.txt", "--mapping-file", "pipelines.yaml", "--dry-run", "--output", "json", "--continue-on-error=false"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !opts.DryRun || opts.Output != "json" || opts.ContinueOnError {
		t.Fatalf("opts=%+v", opts)
	}
}

func TestValidateRunOptions(t *testing.T) {
	base := RunOptions{ImagesFile: "images.txt", MappingFile: "pipelines.yaml", Output: "text", Concurrency: 1, RunPipelineQPM: 100, ContinueOnError: true}
	tests := []struct {
		name string
		mut  func(*RunOptions)
	}{
		{name: "missing images", mut: func(o *RunOptions) { o.ImagesFile = "" }},
		{name: "missing mapping", mut: func(o *RunOptions) { o.MappingFile = "" }},
		{name: "bad output", mut: func(o *RunOptions) { o.Output = "xml" }},
		{name: "bad concurrency", mut: func(o *RunOptions) { o.Concurrency = 2 }},
		{name: "qpm low", mut: func(o *RunOptions) { o.RunPipelineQPM = 0 }},
		{name: "qpm high", mut: func(o *RunOptions) { o.RunPipelineQPM = 101 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := base
			tt.mut(&opts)
			if err := ValidateRunOptions(opts); apperrors.CodeOf(err) != apperrors.CodeInvalidConfig {
				t.Fatalf("code=%q err=%v", apperrors.CodeOf(err), err)
			}
		})
	}
}
