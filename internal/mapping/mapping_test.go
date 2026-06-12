package mapping

import (
	"os"
	"path/filepath"
	"testing"

	apperrors "github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/errors"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pipelines.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadAndLookup(t *testing.T) {
	m, err := Load(writeTemp(t, `repositories:
  repo1:
    workspaceId: w-xxx
    pipelineId: p-xxx
    pipelineName: repo1-nydus-convert
`))
	if err != nil {
		t.Fatal(err)
	}
	ref, err := m.Lookup("repo1")
	if err != nil {
		t.Fatal(err)
	}
	if ref.Repository != "repo1" || ref.WorkspaceID != "w-xxx" || ref.PipelineID != "p-xxx" || ref.PipelineName != "repo1-nydus-convert" {
		t.Fatalf("ref=%+v", ref)
	}
}

func TestLookupErrors(t *testing.T) {
	m, err := Load(writeTemp(t, `repositories:
  repo1:
    workspaceId: ""
    pipelineId: p-xxx
  repo2:
    workspaceId: w-yyy
    pipelineId: ""
`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Lookup("missing"); apperrors.CodeOf(err) != apperrors.CodeMappingNotFound {
		t.Fatalf("missing code=%q err=%v", apperrors.CodeOf(err), err)
	}
	if _, err := m.Lookup("repo1"); apperrors.CodeOf(err) != apperrors.CodeInvalidPipelineMapping {
		t.Fatalf("repo1 code=%q err=%v", apperrors.CodeOf(err), err)
	}
	if _, err := m.Lookup("repo2"); apperrors.CodeOf(err) != apperrors.CodeInvalidPipelineMapping {
		t.Fatalf("repo2 code=%q err=%v", apperrors.CodeOf(err), err)
	}
}

func TestLoadInvalid(t *testing.T) {
	if _, err := Load(writeTemp(t, `bad: [`)); apperrors.CodeOf(err) != apperrors.CodeInvalidPipelineMapping {
		t.Fatalf("bad yaml code=%q err=%v", apperrors.CodeOf(err), err)
	}
	if _, err := Load(writeTemp(t, `foo: bar`)); apperrors.CodeOf(err) != apperrors.CodeInvalidPipelineMapping {
		t.Fatalf("missing repos code=%q err=%v", apperrors.CodeOf(err), err)
	}
}
