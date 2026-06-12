package pipeline

import "context"

type Executor interface {
	Run(ctx context.Context, req RunRequest) (*RunResult, error)
}
