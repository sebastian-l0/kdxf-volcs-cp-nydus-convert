package image

import (
	"testing"

	apperrors "github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/errors"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantReg   string
		wantNS    string
		wantRepo  string
		wantTag   string
		wantDig   string
		wantError apperrors.Code
	}{
		{name: "standard", raw: "vfaas-cn-beijing.cr.volces.com/swe/repo1:v1", wantReg: "vfaas-cn-beijing.cr.volces.com", wantNS: "swe", wantRepo: "repo1", wantTag: "v1"},
		{name: "registry port", raw: "registry.example.com:5000/ns/repo:v1", wantReg: "registry.example.com:5000", wantNS: "ns", wantRepo: "repo", wantTag: "v1"},
		{name: "multi namespace", raw: "registry.example.com/team/group/repo3:v1", wantReg: "registry.example.com", wantNS: "team/group", wantRepo: "repo3", wantTag: "v1"},
		{name: "tag digest", raw: "registry.example.com/ns/repo:v1@sha256:abc", wantReg: "registry.example.com", wantNS: "ns", wantRepo: "repo", wantTag: "v1", wantDig: "sha256:abc"},
		{name: "missing tag", raw: "registry.example.com/ns/repo", wantError: apperrors.CodeTagNotFound},
		{name: "empty tag", raw: "registry.example.com/ns/repo:", wantError: apperrors.CodeTagNotFound},
		{name: "digest only", raw: "registry.example.com/ns/repo@sha256:abc", wantError: apperrors.CodeTagNotFound},
		{name: "no path", raw: "repo:v1", wantError: apperrors.CodeInvalidImageRef},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.raw)
			if tt.wantError != "" {
				if apperrors.CodeOf(err) != tt.wantError {
					t.Fatalf("CodeOf(err)=%q, want %q, err=%v", apperrors.CodeOf(err), tt.wantError, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if got.Registry != tt.wantReg || got.Namespace != tt.wantNS || got.Repository != tt.wantRepo || got.Tag != tt.wantTag || got.Digest != tt.wantDig {
				t.Fatalf("Parse()=%+v", got)
			}
		})
	}
}
