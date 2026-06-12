package pipeline

import "time"

type ImageRef struct {
	Raw        string
	Registry   string
	Namespace  string
	Repository string
	Tag        string
	Digest     string
}

type PipelineRef struct {
	Repository   string
	WorkspaceID  string
	PipelineID   string
	PipelineName string
}

type RunRequest struct {
	Image       ImageRef
	Pipeline    PipelineRef
	DynamicVars map[string]string
	DryRun      bool
}

type RunResult struct {
	ExecutionRecordID string
	Status            string
	TriggeredAt       time.Time
	RawResponse       []byte
}
