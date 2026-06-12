package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	apperrors "github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/errors"
	"github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/pipeline"
)

type mockExecutor struct {
	calls []pipeline.RunRequest
	ids   []string
	err   error
}

func (m *mockExecutor) Run(ctx context.Context, req pipeline.RunRequest) (*pipeline.RunResult, error) {
	m.calls = append(m.calls, req)
	if m.err != nil {
		return nil, m.err
	}
	id := "exec-1"
	if len(m.ids) >= len(m.calls) {
		id = m.ids[len(m.calls)-1]
	}
	return &pipeline.RunResult{ExecutionRecordID: id, Status: "triggered", TriggeredAt: time.Now()}, nil
}

type mockLimiter struct{ calls int }

func (m *mockLimiter) Wait(ctx context.Context) (time.Duration, error) {
	m.calls++
	return time.Duration(m.calls-1) * 600 * time.Millisecond, nil
}

func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func testMapping(t *testing.T) string {
	return writeFile(t, "pipelines.yaml", `repositories:
  repo1:
    workspaceId: w-xxx
    pipelineId: p-xxx
    pipelineName: repo1-nydus-convert
  repo2:
    workspaceId: w-yyy
    pipelineId: p-yyy
    pipelineName: repo2-nydus-convert
`)
}

func TestRunnerDryRun(t *testing.T) {
	images := writeFile(t, "images.txt", "# comment\nvfaas-cn-beijing.cr.volces.com/swe/repo1:h2database__h2database-2346-nydus\n")
	exec := &mockExecutor{}
	limiter := &mockLimiter{}
	res, err := (&Runner{Executor: exec, Limiter: limiter}).Run(context.Background(), RunOptions{ImagesFile: images, MappingFile: testMapping(t), DryRun: true, Output: "json", Concurrency: 1, RunPipelineQPM: 100, ContinueOnError: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(exec.calls) != 0 || limiter.calls != 0 {
		t.Fatalf("dry-run should not call executor/limiter")
	}
	if res.Summary.Total != 1 || res.Summary.Success != 1 || res.Summary.Failed != 0 || len(res.Items) != 1 {
		t.Fatalf("res=%+v", res)
	}
	item := res.Items[0]
	if item.Status != "dry_run" || item.Repository != "repo1" || item.Tag != "h2database__h2database-2346-nydus" || item.WorkspaceID != "w-xxx" || item.PipelineID != "p-xxx" {
		t.Fatalf("item=%+v", item)
	}
	if item.DynamicVars["imageToConvert"] != item.ImageToConvert || item.DynamicVars["tag"] != item.Tag {
		t.Fatalf("dynamic vars=%+v item=%+v", item.DynamicVars, item)
	}
}

func TestRunnerContinueOnError(t *testing.T) {
	images := writeFile(t, "images.txt", "vfaas-cn-beijing.cr.volces.com/swe/missing:v1\nvfaas-cn-beijing.cr.volces.com/swe/repo1:v1\n")
	res, err := (&Runner{}).Run(context.Background(), RunOptions{ImagesFile: images, MappingFile: testMapping(t), DryRun: true, Output: "json", Concurrency: 1, RunPipelineQPM: 100, ContinueOnError: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.Failed != 1 || res.Summary.Success != 1 || len(res.Items) != 2 {
		t.Fatalf("res=%+v", res)
	}
	if res.Items[0].Error == nil || res.Items[0].Error.Code != string(apperrors.CodeMappingNotFound) {
		t.Fatalf("first=%+v", res.Items[0])
	}
}

func TestRunnerStopOnError(t *testing.T) {
	images := writeFile(t, "images.txt", "vfaas-cn-beijing.cr.volces.com/swe/missing:v1\nvfaas-cn-beijing.cr.volces.com/swe/repo1:v1\n")
	res, err := (&Runner{}).Run(context.Background(), RunOptions{ImagesFile: images, MappingFile: testMapping(t), DryRun: true, Output: "json", Concurrency: 1, RunPipelineQPM: 100, ContinueOnError: false})
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.Failed != 1 || res.Summary.Success != 0 || len(res.Items) != 1 {
		t.Fatalf("res=%+v", res)
	}
}

func TestRunnerNonDryRunSuccess(t *testing.T) {
	images := writeFile(t, "images.txt", "vfaas-cn-beijing.cr.volces.com/swe/repo1:v1\nvfaas-cn-beijing.cr.volces.com/swe/repo2:v2\n")
	exec := &mockExecutor{ids: []string{"exec-1", "exec-2"}}
	limiter := &mockLimiter{}
	res, err := (&Runner{Executor: exec, Limiter: limiter}).Run(context.Background(), RunOptions{ImagesFile: images, MappingFile: testMapping(t), DryRun: false, Output: "json", Concurrency: 1, RunPipelineQPM: 100, ContinueOnError: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.Success != 2 || res.Summary.RunPipelineCalls != 2 || len(exec.calls) != 2 || limiter.calls != 2 {
		t.Fatalf("res=%+v exec=%d limiter=%d", res, len(exec.calls), limiter.calls)
	}
	if res.Items[0].RunID != "exec-1" || res.Items[1].RunID != "exec-2" {
		t.Fatalf("items=%+v", res.Items)
	}
	if exec.calls[0].DynamicVars["imageToConvert"] != res.Items[0].ImageToConvert || exec.calls[0].DynamicVars["tag"] != "v1" {
		t.Fatalf("call=%+v", exec.calls[0])
	}
}

func TestRunnerEmptyExecutionRecordID(t *testing.T) {
	images := writeFile(t, "images.txt", "vfaas-cn-beijing.cr.volces.com/swe/repo1:v1\n")
	exec := &mockExecutor{ids: []string{""}}
	res, err := (&Runner{Executor: exec, Limiter: &mockLimiter{}}).Run(context.Background(), RunOptions{ImagesFile: images, MappingFile: testMapping(t), DryRun: false, Output: "json", Concurrency: 1, RunPipelineQPM: 100, ContinueOnError: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.Failed != 1 || res.Items[0].Error == nil || res.Items[0].Error.Code != string(apperrors.CodeRunPipelineResponseInvalid) {
		t.Fatalf("res=%+v", res)
	}
}
