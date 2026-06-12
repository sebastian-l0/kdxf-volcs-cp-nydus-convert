package volcengine

import (
	"context"
	"errors"
	"testing"

	cp "github.com/volcengine/volcengine-go-sdk/service/cp"
	volcsdk "github.com/volcengine/volcengine-go-sdk/volcengine"
	"github.com/volcengine/volcengine-go-sdk/volcengine/request"

	apperrors "github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/errors"
	"github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/pipeline"
)

type mockCPClient struct {
	input *cp.RunPipelineInput
	out   *cp.RunPipelineOutput
	err   error
}

func (m *mockCPClient) RunPipelineWithContext(ctx volcsdk.Context, input *cp.RunPipelineInput, opts ...request.Option) (*cp.RunPipelineOutput, error) {
	m.input = input
	return m.out, m.err
}

func TestBuildRunPipelineInput(t *testing.T) {
	input := BuildRunPipelineInput(pipeline.RunRequest{
		Pipeline: pipeline.PipelineRef{WorkspaceID: "workspace-1", PipelineID: "pipeline-1"},
		DynamicVars: map[string]string{
			"imageToConvert": "registry/ns/repo:v1",
			"tag":            "v1",
		},
	})
	if input.WorkspaceId == nil || *input.WorkspaceId != "workspace-1" {
		t.Fatalf("WorkspaceId=%v", input.WorkspaceId)
	}
	if input.Id == nil || *input.Id != "pipeline-1" {
		t.Fatalf("Id=%v", input.Id)
	}
	got := map[string]string{}
	for _, p := range input.Parameters {
		if p.Key != nil && p.Value != nil {
			got[*p.Key] = *p.Value
		}
	}
	if got["imageToConvert"] != "registry/ns/repo:v1" || got["tag"] != "v1" {
		t.Fatalf("parameters=%v", got)
	}
}

func TestExecutorRunSuccess(t *testing.T) {
	id := "exec-123"
	mock := &mockCPClient{out: (&cp.RunPipelineOutput{}).SetId(id)}
	res, err := NewExecutorWithClient(mock).Run(context.Background(), pipeline.RunRequest{
		Pipeline:    pipeline.PipelineRef{WorkspaceID: "workspace-1", PipelineID: "pipeline-1"},
		DynamicVars: map[string]string{"imageToConvert": "image", "tag": "tag"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ExecutionRecordID != id || mock.input == nil {
		t.Fatalf("res=%+v input=%+v", res, mock.input)
	}
}

func TestExecutorRunErrors(t *testing.T) {
	_, err := NewExecutorWithClient(&mockCPClient{err: errors.New("boom")}).Run(context.Background(), pipeline.RunRequest{})
	if apperrors.CodeOf(err) != apperrors.CodeRunPipelineFailed {
		t.Fatalf("code=%q err=%v", apperrors.CodeOf(err), err)
	}

	_, err = NewExecutorWithClient(&mockCPClient{out: &cp.RunPipelineOutput{}}).Run(context.Background(), pipeline.RunRequest{})
	if apperrors.CodeOf(err) != apperrors.CodeRunPipelineResponseInvalid {
		t.Fatalf("code=%q err=%v", apperrors.CodeOf(err), err)
	}
}
