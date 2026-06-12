package volcengine

import (
	"context"
	"time"

	cp "github.com/volcengine/volcengine-go-sdk/service/cp"
	volcsdk "github.com/volcengine/volcengine-go-sdk/volcengine"
	"github.com/volcengine/volcengine-go-sdk/volcengine/credentials"
	"github.com/volcengine/volcengine-go-sdk/volcengine/request"
	"github.com/volcengine/volcengine-go-sdk/volcengine/session"

	apperrors "github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/errors"
	"github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/pipeline"
)

type Options struct {
	Region    string
	AccessKey string
	SecretKey string
}

type sdkClient interface {
	RunPipelineWithContext(ctx volcsdk.Context, input *cp.RunPipelineInput, opts ...request.Option) (*cp.RunPipelineOutput, error)
}

type Executor struct {
	client  sdkClient
	initErr error
}

func NewExecutor(opts Options) *Executor {
	region := opts.Region
	if region == "" {
		region = "cn-beijing"
	}
	config := volcsdk.NewConfig().WithRegion(region)
	if opts.AccessKey != "" || opts.SecretKey != "" {
		config.WithCredentials(credentials.NewStaticCredentials(opts.AccessKey, opts.SecretKey, ""))
	} else {
		config.WithCredentials(credentials.NewEnvCredentials())
	}
	sess, err := session.NewSession(config)
	if err != nil {
		return &Executor{initErr: err}
	}
	return &Executor{client: cp.New(sess)}
}

func NewExecutorWithClient(client sdkClient) *Executor {
	return &Executor{client: client}
}

func (e *Executor) Run(ctx context.Context, req pipeline.RunRequest) (*pipeline.RunResult, error) {
	if e.initErr != nil {
		return nil, apperrors.Wrap(apperrors.CodeInvalidConfig, "failed to initialize Volcengine CP client", e.initErr)
	}
	client := e.client
	if client == nil {
		return nil, apperrors.New(apperrors.CodeRunPipelineNotImplemented, "volcengine RunPipeline executor is not configured")
	}
	input := BuildRunPipelineInput(req)
	out, err := client.RunPipelineWithContext(ctx, input)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeRunPipelineFailed, "RunPipeline request failed", err)
	}
	if out == nil || out.Id == nil || *out.Id == "" {
		return nil, apperrors.New(apperrors.CodeRunPipelineResponseInvalid, "RunPipeline response missing execution record id")
	}
	return &pipeline.RunResult{ExecutionRecordID: *out.Id, Status: "triggered", TriggeredAt: time.Now()}, nil
}

func BuildRunPipelineInput(req pipeline.RunRequest) *cp.RunPipelineInput {
	params := make([]*cp.ParameterForRunPipelineInput, 0, len(req.DynamicVars))
	for k, v := range req.DynamicVars {
		params = append(params, (&cp.ParameterForRunPipelineInput{}).SetKey(k).SetValue(v))
	}
	return (&cp.RunPipelineInput{}).
		SetWorkspaceId(req.Pipeline.WorkspaceID).
		SetId(req.Pipeline.PipelineID).
		SetParameters(params)
}
