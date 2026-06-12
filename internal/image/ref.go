package image

import (
	"strings"

	apperrors "github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/errors"
)

type Ref struct {
	Raw        string `json:"raw"`
	Registry   string `json:"registry"`
	Namespace  string `json:"namespace,omitempty"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Digest     string `json:"digest,omitempty"`
}

func Parse(raw string) (Ref, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, " \t\n\r") {
		return Ref{}, apperrors.New(apperrors.CodeInvalidImageRef, "invalid image reference")
	}

	namePart := raw
	digest := ""
	if at := strings.Index(namePart, "@"); at >= 0 {
		digest = namePart[at+1:]
		namePart = namePart[:at]
		if digest == "" || namePart == "" {
			return Ref{}, apperrors.New(apperrors.CodeInvalidImageRef, "invalid image digest reference")
		}
	}

	firstSlash := strings.Index(namePart, "/")
	if firstSlash <= 0 || firstSlash == len(namePart)-1 {
		return Ref{}, apperrors.New(apperrors.CodeInvalidImageRef, "image reference must include registry and repository path")
	}
	registry := namePart[:firstSlash]
	pathPart := namePart[firstSlash+1:]

	lastSlash := strings.LastIndex(pathPart, "/")
	lastSegment := pathPart
	namespace := ""
	if lastSlash >= 0 {
		namespace = pathPart[:lastSlash]
		lastSegment = pathPart[lastSlash+1:]
	}
	if lastSegment == "" {
		return Ref{}, apperrors.New(apperrors.CodeInvalidImageRef, "repository is empty")
	}

	colon := strings.LastIndex(lastSegment, ":")
	if colon < 0 || colon == len(lastSegment)-1 {
		return Ref{}, apperrors.New(apperrors.CodeTagNotFound, "image tag is required")
	}
	repo := lastSegment[:colon]
	tag := lastSegment[colon+1:]
	if repo == "" {
		return Ref{}, apperrors.New(apperrors.CodeInvalidImageRef, "repository is empty")
	}

	return Ref{Raw: raw, Registry: registry, Namespace: namespace, Repository: repo, Tag: tag, Digest: digest}, nil
}
