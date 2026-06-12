package mapping

import (
	"os"
	"strings"

	apperrors "github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/errors"
	"gopkg.in/yaml.v3"
)

type PipelineRef struct {
	Repository   string `json:"repository" yaml:"-"`
	WorkspaceID  string `json:"workspaceId" yaml:"workspaceId"`
	PipelineID   string `json:"pipelineId" yaml:"pipelineId"`
	PipelineName string `json:"pipelineName,omitempty" yaml:"pipelineName"`
}

type file struct {
	Repositories map[string]PipelineRef `yaml:"repositories"`
}

type Mapping struct {
	repositories map[string]PipelineRef
}

func Load(path string) (*Mapping, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, apperrors.Wrap(apperrors.CodeMappingFileNotFound, "mapping file not found", err)
		}
		return nil, apperrors.Wrap(apperrors.CodeMappingFileNotFound, "failed to read mapping file", err)
	}
	var f file
	if err := yaml.Unmarshal(b, &f); err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInvalidPipelineMapping, "failed to parse mapping YAML", err)
	}
	if len(f.Repositories) == 0 {
		return nil, apperrors.New(apperrors.CodeInvalidPipelineMapping, "mapping file must contain repositories")
	}
	m := &Mapping{repositories: make(map[string]PipelineRef, len(f.Repositories))}
	for repo, ref := range f.Repositories {
		repo = strings.TrimSpace(repo)
		ref.Repository = repo
		m.repositories[repo] = ref
	}
	return m, nil
}

func (m *Mapping) Lookup(repository string) (PipelineRef, error) {
	if m == nil {
		return PipelineRef{}, apperrors.New(apperrors.CodeMappingNotFound, "pipeline mapping is not loaded")
	}
	ref, ok := m.repositories[repository]
	if !ok {
		return PipelineRef{}, apperrors.New(apperrors.CodeMappingNotFound, "pipeline mapping not found for repository "+repository)
	}
	if strings.TrimSpace(ref.WorkspaceID) == "" || strings.TrimSpace(ref.PipelineID) == "" {
		return PipelineRef{}, apperrors.New(apperrors.CodeInvalidPipelineMapping, "mapping for repository "+repository+" must include workspaceId and pipelineId")
	}
	return ref, nil
}
