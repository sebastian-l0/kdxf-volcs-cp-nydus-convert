package app

import (
	"context"
	"os"

	apperrors "github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/errors"
	"github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/image"
	"github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/input"
	"github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/mapping"
	"github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/pipeline"
	"github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/ratelimit"
	"github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/volcengine"
)

type RunOptions struct {
	ImagesFile      string
	MappingFile     string
	DryRun          bool
	Output          string
	Region          string
	AccessKey       string
	SecretKey       string
	Concurrency     int
	RunPipelineQPM  int
	ContinueOnError bool
}

type Runner struct {
	Executor pipeline.Executor
	Limiter  ratelimit.Limiter
}

type Result struct {
	DryRun  bool         `json:"dry_run"`
	Summary Summary      `json:"summary"`
	Items   []ResultItem `json:"items"`
}

type Summary struct {
	Total            int `json:"total"`
	Success          int `json:"success"`
	Failed           int `json:"failed"`
	Skipped          int `json:"skipped"`
	RunPipelineCalls int `json:"run_pipeline_calls"`
	RunPipelineQPM   int `json:"run_pipeline_qpm"`
}

type ResultItem struct {
	Line            int               `json:"line"`
	ImageToConvert  string            `json:"imageToConvert"`
	Registry        string            `json:"registry,omitempty"`
	Namespace       string            `json:"namespace,omitempty"`
	Repository      string            `json:"repository,omitempty"`
	Tag             string            `json:"tag,omitempty"`
	WorkspaceID     string            `json:"workspaceId,omitempty"`
	PipelineID      string            `json:"pipelineId,omitempty"`
	PipelineName    string            `json:"pipelineName,omitempty"`
	Status          string            `json:"status"`
	RunID           string            `json:"run_id,omitempty"`
	RateLimitWaitMS int64             `json:"rate_limit_wait_ms,omitempty"`
	DynamicVars     map[string]string `json:"dynamic_vars,omitempty"`
	Error           *ItemError        `json:"error,omitempty"`
}

type ItemError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (r *Runner) Run(ctx context.Context, opts RunOptions) (*Result, error) {
	lines, err := input.LoadImageLines(opts.ImagesFile)
	if err != nil {
		return nil, err
	}
	m, err := mapping.Load(opts.MappingFile)
	if err != nil {
		return nil, err
	}

	res := &Result{DryRun: opts.DryRun, Summary: Summary{Total: len(lines), RunPipelineQPM: opts.RunPipelineQPM}}
	for _, line := range lines {
		item := ResultItem{Line: line.LineNumber, ImageToConvert: line.Raw}

		ref, err := image.Parse(line.Raw)
		if err != nil {
			recordFailure(res, item, err)
			if !opts.ContinueOnError {
				break
			}
			continue
		}
		fillImage(&item, ref)

		pipe, err := m.Lookup(ref.Repository)
		if err != nil {
			recordFailure(res, item, err)
			if !opts.ContinueOnError {
				break
			}
			continue
		}
		fillPipeline(&item, pipe)

		dynamicVars := map[string]string{"imageToConvert": ref.Raw, "tag": ref.Tag}
		if opts.DryRun {
			item.Status = "dry_run"
			item.DynamicVars = dynamicVars
			recordSuccess(res, item)
			continue
		}

		if r.Limiter == nil {
			limiter, err := ratelimit.NewFixedIntervalLimiter(opts.RunPipelineQPM)
			if err != nil {
				return nil, err
			}
			r.Limiter = limiter
		}
		waited, err := r.Limiter.Wait(ctx)
		if err != nil {
			recordFailure(res, item, err)
			break
		}

		exec := r.Executor
		if exec == nil {
			exec = defaultExecutor(opts)
		}
		runResult, err := exec.Run(ctx, pipeline.RunRequest{Image: toPipelineImage(ref), Pipeline: toPipelineRef(pipe), DynamicVars: dynamicVars})
		if err != nil {
			recordFailure(res, item, err)
			if !opts.ContinueOnError {
				break
			}
			continue
		}
		if runResult == nil || runResult.ExecutionRecordID == "" {
			recordFailure(res, item, apperrors.New(apperrors.CodeRunPipelineResponseInvalid, "RunPipeline response missing execution record id"))
			if !opts.ContinueOnError {
				break
			}
			continue
		}
		item.Status = "triggered"
		item.RunID = runResult.ExecutionRecordID
		item.RateLimitWaitMS = waited.Milliseconds()
		res.Summary.RunPipelineCalls++
		recordSuccess(res, item)
	}
	return res, nil
}

func defaultExecutor(opts RunOptions) pipeline.Executor {
	region := firstNonEmpty(opts.Region, os.Getenv("VOLCENGINE_REGION"), "cn-beijing")
	ak := firstNonEmpty(opts.AccessKey, os.Getenv("VOLCENGINE_ACCESS_KEY"), os.Getenv("VOLCENGINE_ACCESS_KEY_ID"))
	sk := firstNonEmpty(opts.SecretKey, os.Getenv("VOLCENGINE_SECRET_KEY"), os.Getenv("VOLCENGINE_SECRET_ACCESS_KEY"))
	return volcengine.NewExecutor(volcengine.Options{Region: region, AccessKey: ak, SecretKey: sk})
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func recordSuccess(res *Result, item ResultItem) {
	res.Summary.Success++
	res.Items = append(res.Items, item)
}

func recordFailure(res *Result, item ResultItem, err error) {
	res.Summary.Failed++
	item.Status = "failed"
	item.Error = &ItemError{Code: string(apperrors.CodeOf(err)), Message: apperrors.MessageOf(err)}
	res.Items = append(res.Items, item)
}

func fillImage(item *ResultItem, ref image.Ref) {
	item.Registry = ref.Registry
	item.Namespace = ref.Namespace
	item.Repository = ref.Repository
	item.Tag = ref.Tag
}

func fillPipeline(item *ResultItem, ref mapping.PipelineRef) {
	item.WorkspaceID = ref.WorkspaceID
	item.PipelineID = ref.PipelineID
	item.PipelineName = ref.PipelineName
}

func toPipelineImage(ref image.Ref) pipeline.ImageRef {
	return pipeline.ImageRef{Raw: ref.Raw, Registry: ref.Registry, Namespace: ref.Namespace, Repository: ref.Repository, Tag: ref.Tag, Digest: ref.Digest}
}

func toPipelineRef(ref mapping.PipelineRef) pipeline.PipelineRef {
	return pipeline.PipelineRef{Repository: ref.Repository, WorkspaceID: ref.WorkspaceID, PipelineID: ref.PipelineID, PipelineName: ref.PipelineName}
}
