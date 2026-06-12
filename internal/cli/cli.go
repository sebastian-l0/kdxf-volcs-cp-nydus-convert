package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/app"
	apperrors "github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/errors"
	"github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/output"
)

type RunOptions = app.RunOptions

func Main(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "usage: nydus-convert run --images-file <file> --mapping-file <file>")
		return 1
	}
	if args[0] != "run" {
		_, _ = fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		return 1
	}
	opts, err := ParseRunOptions(args[1:], stderr)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err.Error())
		return 1
	}
	runner := &app.Runner{}
	res, err := runner.Run(ctx, opts)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err.Error())
		return 1
	}
	if err := output.Write(stdout, opts.Output, res); err != nil {
		_, _ = fmt.Fprintln(stderr, err.Error())
		return 1
	}
	if res.Summary.Failed > 0 {
		return 1
	}
	return 0
}

func ParseRunOptions(args []string, errOut io.Writer) (RunOptions, error) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(errOut)
	opts := RunOptions{Output: "text", Concurrency: 1, RunPipelineQPM: 100, ContinueOnError: true}
	fs.StringVar(&opts.ImagesFile, "images-file", "", "image list file")
	fs.StringVar(&opts.MappingFile, "mapping-file", "", "repository to pipeline mapping file")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "print request summary without triggering pipelines")
	fs.StringVar(&opts.Output, "output", "text", "output format: text or json")
	fs.StringVar(&opts.Region, "region", "", "Volcengine region, defaults to VOLCENGINE_REGION or cn-beijing")
	fs.StringVar(&opts.AccessKey, "ak", "", "Volcengine access key, defaults to VOLCENGINE_ACCESS_KEY or VOLCENGINE_ACCESS_KEY_ID")
	fs.StringVar(&opts.SecretKey, "sk", "", "Volcengine secret key, defaults to VOLCENGINE_SECRET_KEY or VOLCENGINE_SECRET_ACCESS_KEY")
	fs.IntVar(&opts.Concurrency, "concurrency", 1, "must be 1 in MVP")
	fs.IntVar(&opts.RunPipelineQPM, "run-pipeline-qpm", 100, "max RunPipeline calls per minute, 1-100")
	fs.BoolVar(&opts.ContinueOnError, "continue-on-error", true, "continue processing after per-item errors")
	if err := fs.Parse(args); err != nil {
		return opts, apperrors.Wrap(apperrors.CodeInvalidConfig, "failed to parse arguments", err)
	}
	if err := ValidateRunOptions(opts); err != nil {
		return opts, err
	}
	return opts, nil
}

func ValidateRunOptions(opts RunOptions) error {
	if opts.ImagesFile == "" {
		return apperrors.New(apperrors.CodeInvalidConfig, "--images-file is required")
	}
	if opts.MappingFile == "" {
		return apperrors.New(apperrors.CodeInvalidConfig, "--mapping-file is required")
	}
	if opts.Output != "text" && opts.Output != "json" {
		return apperrors.New(apperrors.CodeInvalidConfig, "--output must be text or json")
	}
	if opts.Concurrency != 1 {
		return apperrors.New(apperrors.CodeInvalidConfig, "--concurrency must be 1 in MVP")
	}
	if opts.RunPipelineQPM < 1 || opts.RunPipelineQPM > 100 {
		return apperrors.New(apperrors.CodeInvalidConfig, "--run-pipeline-qpm must be between 1 and 100")
	}
	return nil
}
