package pipeline

import (
	"context"

	apperrors "github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/errors"
)

type NotImplementedExecutor struct{}

func (NotImplementedExecutor) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	return nil, apperrors.New(apperrors.CodeRunPipelineNotImplemented, "real RunPipeline executor is not implemented yet; use --dry-run for MVP")
}
